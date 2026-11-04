package rules

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"maps"
	"path"
	"slices"

	"github.com/m7medvision/go-clean-template/tools/gen/edit"
	"github.com/m7medvision/go-clean-template/tools/gen/names"
	"github.com/m7medvision/go-clean-template/tools/gen/plan"
)

// DomainFree checks the rest of R1 for a new domain: nothing in the repository uses its name yet.
// That means no use-case or read-model folder, no entity file, no table, no proto service and no
// GraphQL type. The pattern and keyword checks are names.Parse's.
func DomainFree(r Repo, n *names.Name) error {
	for _, p := range []string{
		"internal/usecase/" + n.Pkg,
		"internal/repo/persistent/" + n.Pkg,
		"internal/entity/" + n.Snake + ".go",
		"internal/entity/" + n.Snake + "_aggregate.go",
		"internal/app/schema/" + n.Plural.Snake + ".hcl",
	} {
		if r.exists(p) {
			return taken(n, p+" exists")
		}
	}

	for _, p := range r.glob("docs/proto/v1/*.proto") {
		if pf, err := parsed(r, p, edit.ParseProto); err == nil && slices.Contains(pf.Services(), n.Go+"Service") {
			return taken(n, p+" declares service "+n.Go+"Service")
		}
	}

	for _, p := range r.glob("internal/controller/graphql/schema/*.graphqls") {
		if q, err := parsed(r, p, edit.ParseGraphQL); err == nil && slices.Contains(q.Types(), n.Go) {
			return taken(n, p+" declares type "+n.Go)
		}
	}

	return nil
}

func taken(n *names.Name, why string) error {
	return fmt.Errorf("%w: %q is taken: %s", names.ErrInvalid, n.Kebab, why)
}

// parsed reads the file at p and parses it with parse.
func parsed[T any](r Repo, p string, parse func(string, []byte) (T, error)) (T, error) {
	src, err := r.Read(p)
	if err != nil {
		var zero T

		return zero, err
	}

	return parse(p, src)
}

// IdentifiersFree checks the last of R1, on the plan made with the name n: no identifier the plan
// adds at the top level of a Go package, in a file it creates or by an edit, is declared in that
// package already. A name can be free of files and still clash: record would declare the type
// entity.Record, and the entity package has a function of that name. Methods count as
// Type.Method. Changes that do not format or parse are left to R5.
func IdentifiersFree(r Repo, n *names.Name, p *plan.Plan) error {
	changes, err := p.Changes()
	if err != nil {
		return nil // R5 reports the change that breaks
	}

	byDir := map[string]map[string]plan.Change{}

	for _, c := range changes {
		if path.Ext(c.Path) != ".go" {
			continue
		}

		dir := path.Dir(c.Path)
		if byDir[dir] == nil {
			byDir[dir] = map[string]plan.Change{}
		}

		byDir[dir][c.Path] = c
	}

	for _, dir := range slices.Sorted(maps.Keys(byDir)) {
		if err := packageFree(r, n, dir, byDir[dir]); err != nil {
			return err
		}
	}

	return nil
}

// declared is one top-level identifier of one package, which several files can share as test
// packages do.
type declared struct {
	pkg, name string
}

// packageFree checks the identifiers that planned, the changes to the Go files in dir, add against
// every Go file in dir as the plan leaves it.
func packageFree(r Repo, n *names.Name, dir string, planned map[string]plan.Change) error {
	var err error

	files := slices.Collect(maps.Keys(planned))
	files = append(files, r.glob(path.Join(dir, "*.go"))...)
	slices.Sort(files)
	files = slices.Compact(files)

	where := map[declared][]string{}

	for _, f := range files {
		src := planned[f].New
		if _, ok := planned[f]; !ok {
			// A file that cannot be read declares nothing here; the build would report it anyway.
			if src, err = fs.ReadFile(r.FS, f); err != nil {
				continue
			}
		}

		pkg, idents := topLevel(f, src)
		for _, id := range idents {
			where[declared{pkg, id}] = append(where[declared{pkg, id}], f)
		}
	}

	for _, f := range slices.Sorted(maps.Keys(planned)) {
		pkg, idents := added(planned[f])

		for _, id := range idents {
			if files := where[declared{pkg, id}]; len(files) > 1 {
				return taken(n, clash(f, id, files))
			}
		}
	}

	return nil
}

// clash says that f would declare id, which files, f among them, all declare.
func clash(f, id string, files []string) string {
	for _, other := range files {
		if other != f {
			return fmt.Sprintf("%s would declare %s, which %s declares already", f, id, other)
		}
	}

	return fmt.Sprintf("%s would declare %s twice", f, id)
}

// added returns the package of a changed Go file and the identifiers the change adds to it: every
// one of a created file, and those an edit declares more often than the file did.
func added(c plan.Change) (pkg string, adds []string) {
	pkg, after := topLevel(c.Path, c.New)
	_, before := topLevel(c.Path, c.Old)

	for _, id := range after {
		if i := slices.Index(before, id); i >= 0 {
			before = slices.Delete(before, i, i+1)

			continue
		}

		adds = append(adds, id)
	}

	return pkg, adds
}

// topLevel returns the package src declares and its top-level identifiers, methods as
// Type.Method, in source order; nothing for src that is empty or does not parse. The blank
// identifier and init, which a package may declare more than once, are left out.
func topLevel(p string, src []byte) (pkg string, idents []string) {
	if src == nil {
		return "", nil
	}

	file, err := parser.ParseFile(token.NewFileSet(), p, src, parser.SkipObjectResolution)
	if err != nil {
		return "", nil
	}

	for _, d := range file.Decls {
		for _, id := range declNames(d) {
			if id != "_" && id != "init" {
				idents = append(idents, id)
			}
		}
	}

	return file.Name.Name, idents
}

// declNames are the identifiers one top-level declaration declares.
func declNames(d ast.Decl) []string {
	var ids []string

	switch d := d.(type) {
	case *ast.FuncDecl:
		if d.Recv == nil || len(d.Recv.List) == 0 {
			return []string{d.Name.Name}
		}

		return []string{receiverType(d.Recv.List[0].Type) + "." + d.Name.Name}
	case *ast.GenDecl:
		for _, s := range d.Specs {
			switch s := s.(type) {
			case *ast.TypeSpec:
				ids = append(ids, s.Name.Name)
			case *ast.ValueSpec:
				for _, id := range s.Names {
					ids = append(ids, id.Name)
				}
			}
		}
	}

	return ids
}

// receiverType is the name of a method's receiver type: T for T, *T, T[K] and *T[K].
func receiverType(e ast.Expr) string {
	switch t := e.(type) {
	case *ast.StarExpr:
		return receiverType(t.X)
	case *ast.IndexExpr:
		return receiverType(t.X)
	case *ast.IndexListExpr:
		return receiverType(t.X)
	case *ast.Ident:
		return t.Name
	default:
		return ""
	}
}
