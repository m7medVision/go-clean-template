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
// That means no use-case or read-model folder, no entity file, no table, no controller folder on
// any transport, no proto service and no GraphQL type. A controller folder can be a package the
// domains share, such as the REST response package, which the domain's controllers would join.
// The pattern and keyword checks are names.Parse's.
func DomainFree(r Repo, n *names.Name) error {
	owned := []string{
		"internal/usecase/" + n.Pkg,
		"internal/repo/persistent/" + n.Pkg,
		"internal/entity/" + n.Snake + ".go",
		"internal/entity/" + n.Snake + "_aggregate.go",
		"internal/app/schema/" + n.Plural.Snake + ".hcl",
	}

	for _, t := range Transports() {
		owned = append(owned, transportDir(t, n.Pkg))
	}

	for _, p := range owned {
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

// IdentifiersFree checks the last of R1, on the plan made with the name n: no name the plan adds
// to a package, in a file it creates or by an edit, is declared in that package already. A name
// can be free of files and still clash. In Go the names are the top-level identifiers, methods as
// Type.Method: record would declare the type entity.Record, and the entity package has a function
// of that name. In a .proto file they are its messages and services, and its services' methods as
// Service.Method, and the package is the folder, as docs/proto/v1 is grpc.v1: login would declare
// LoginResponse, which auth.proto declares, and only make proto-v1 would find it, after the write.
// In a .graphqls file they are the types it defines and the fields it gives any type, as
// Type.field, and the schema is the folder, which gqlgen reads as one: all-task would add
// Query.allTasks, which task.graphqls has, and only make graphql would find it. Changes that do not
// format or parse are left to R5.
func IdentifiersFree(r Repo, n *names.Name, p *plan.Plan) error {
	changes, err := p.Changes()
	if err != nil {
		return nil // R5 reports the change that breaks
	}

	// The changes by the package they declare into, as the glob of its files: dir/*.go, dir/*.proto,
	// dir/*.graphqls.
	byPackage := map[string]map[string]plan.Change{}

	for _, c := range changes {
		ext := path.Ext(c.Path)
		if declarationsOf(ext) == nil {
			continue
		}

		files := path.Join(path.Dir(c.Path), "*"+ext)
		if byPackage[files] == nil {
			byPackage[files] = map[string]plan.Change{}
		}

		byPackage[files][c.Path] = c
	}

	for _, files := range slices.Sorted(maps.Keys(byPackage)) {
		if err := packageFree(r, n, files, byPackage[files]); err != nil {
			return err
		}
	}

	return nil
}

// declarations returns the package a file declares into and the names it declares there, in
// source order; nothing for src that is empty or does not parse.
type declarations func(p string, src []byte) (pkg string, idents []string)

// declarationsOf returns how to read the names a file with the extension ext declares, or nil for
// a kind of file that declares none a package shares.
func declarationsOf(ext string) declarations {
	switch ext {
	case ".go":
		return topLevel
	case ".proto":
		return protoTopLevel
	case ".graphqls":
		return graphqlTopLevel
	default:
		return nil
	}
}

// declared is one name of one package, which several files can share as test packages do.
type declared struct {
	pkg, name string
}

// packageFree checks the names that planned, the changes to the files matching the glob files, add
// against every file it matches as the plan leaves it.
func packageFree(r Repo, n *names.Name, files string, planned map[string]plan.Change) error {
	var err error

	decls := declarationsOf(path.Ext(files))
	all := slices.Collect(maps.Keys(planned))
	all = append(all, r.glob(files)...)
	slices.Sort(all)
	all = slices.Compact(all)

	where := map[declared][]string{}

	for _, f := range all {
		src := planned[f].New
		if _, ok := planned[f]; !ok {
			// A file that cannot be read declares nothing here; the build would report it anyway.
			if src, err = fs.ReadFile(r.FS, f); err != nil {
				continue
			}
		}

		pkg, ids := decls(f, src)
		for _, id := range ids {
			where[declared{pkg, id}] = append(where[declared{pkg, id}], f)
		}
	}

	for _, f := range slices.Sorted(maps.Keys(planned)) {
		pkg, ids := added(planned[f], decls)

		for _, id := range ids {
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

// added returns the package of a changed file and the names the change adds to it, read with
// decls: every one of a created file, and those an edit declares more often than the file did.
func added(c plan.Change, decls declarations) (pkg string, adds []string) {
	pkg, after := decls(c.Path, c.New)
	_, before := decls(c.Path, c.Old)

	for _, id := range after {
		if i := slices.Index(before, id); i >= 0 {
			before = slices.Delete(before, i, i+1)

			continue
		}

		adds = append(adds, id)
	}

	return pkg, adds
}

// protoTopLevel returns the services and top-level messages the .proto file src declares, which
// share one namespace, and the methods of its services, as Service.Method; nothing for src that is
// empty or does not parse. The package is the folder, whose files are one proto package, so it is
// left empty.
func protoTopLevel(p string, src []byte) (pkg string, idents []string) {
	if src == nil {
		return "", nil
	}

	f, err := edit.ParseProto(p, src)
	if err != nil {
		return "", nil
	}

	return "", slices.Concat(f.Services(), f.Messages(), f.RPCs())
}

// graphqlTopLevel returns the types the .graphqls file src defines and the fields it gives every
// type it defines or extends, as Type.field; nothing for src that is empty or does not parse. A
// type's fields are unique across the files of a schema, which is its folder, so the package is
// left empty.
func graphqlTopLevel(p string, src []byte) (pkg string, idents []string) {
	if src == nil {
		return "", nil
	}

	q, err := edit.ParseGraphQL(p, src)
	if err != nil {
		return "", nil
	}

	return "", slices.Concat(q.Types(), q.Fields())
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
