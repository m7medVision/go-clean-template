package rules

import (
	"context"
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
// any transport, no proto service, no GraphQL type and no package (PackageNameFree). A controller
// folder can be a package the domains share, such as the REST response package, which the domain's
// controllers would join. The pattern and keyword checks are names.Parse's.
func DomainFree(ctx context.Context, r Repo, n *names.Name) error {
	owned := []string{
		"internal/usecase/" + n.Pkg,
		"internal/repo/persistent/" + n.Pkg,
		"internal/entity/" + n.Snake + ".go",
		"internal/entity/" + n.Snake + "_aggregate.go",
		"internal/app/schema/" + n.Plural.Snake + ".hcl",
	}

	for _, t := range Transports() {
		owned = append(owned, ControllerDir(t, n.Pkg))
	}

	for _, p := range owned {
		if r.Exists(p) {
			return taken(n, p+" exists")
		}
	}

	if err := noneDeclares(r, n, ProtoDir+"/*.proto", edit.ParseProto, (*edit.Proto).Services, "service", n.Go+"Service"); err != nil {
		return err
	}

	if err := noneDeclares(r, n, SchemaDir()+"/*.graphqls", edit.ParseGraphQL, (*edit.GraphQL).Types, "type", n.Go); err != nil {
		return err
	}

	return PackageNameFree(ctx, r, n)
}

// noneDeclares checks that no file matching the glob files, parsed with parse, declares the name of
// the kind, which n would declare, among the names of that kind decls reads from it. A file that
// does not parse cannot say, so it fails the check.
func noneDeclares[T any](r Repo, n *names.Name, files string, parse func(string, []byte) (T, error), decls func(T) []string,
	kind, name string,
) error {
	paths, err := r.glob(files)
	if err != nil {
		return err
	}

	for _, p := range paths {
		f, err := parsed(r, p, parse)
		if err != nil {
			return err
		}

		if slices.Contains(decls(f), name) {
			return taken(n, p+" declares "+kind+" "+name)
		}
	}

	return nil
}

func taken(n *names.Name, why string) error {
	return fmt.Errorf("%w: %q is taken: %s", names.ErrInvalid, n.Kebab, why)
}

// PackageNameFree checks R1 on the name n against the app's packages, as go list names them: no
// form of n or of its plural that Go code uses as an identifier, the package form or the lower
// one, is the name of a package. A generated file that imports that package, as a domain's
// controllers import command and query, would find it shadowed.
func PackageNameFree(ctx context.Context, r Repo, n *names.Name) error {
	pkgs, err := r.Packages(ctx)
	if err != nil {
		return fmt.Errorf("list the app's packages: %w", err)
	}

	for _, ident := range []string{n.Pkg, n.Lower, n.Plural.Pkg, n.Plural.Lower} {
		if importPath, ok := pkgs[ident]; ok {
			return fmt.Errorf("%w: %q gives %q, the name of the package %s, which generated files would shadow",
				names.ErrInvalid, n.Kebab, ident, importPath)
		}
	}

	return nil
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
// Query.allTasks, which task.graphqls has, and only make graphql would find it.
//
// The check reads every file as the plan leaves it, unformatted, since what a file declares does
// not depend on its layout. A file the plan leaves broken declares nothing here: R5 reports it in
// its turn, after R2 to R4, since a broken file most often follows from a place R4 did not find.
// Any other file that does not parse fails the check, which cannot tell what it declares.
func IdentifiersFree(r Repo, n *names.Name, p *plan.Plan) error {
	changes := p.Contents()

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
// source order; nothing for src that is nil, as a created file's content before the plan is.
type declarations func(p string, src []byte) (pkg string, idents []string, err error)

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
	decls := declarationsOf(path.Ext(files))

	existing, err := r.glob(files)
	if err != nil {
		return err
	}

	all := slices.Concat(slices.Collect(maps.Keys(planned)), existing)
	slices.Sort(all)
	all = slices.Compact(all)

	where := map[declared][]string{}

	for _, f := range all {
		pkg, ids, err := declaredBy(r, f, planned, decls)
		if err != nil {
			return err
		}

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

// declaredBy returns the package the file at f declares into, as the plan leaves it, and the names
// it declares there, read with decls. A file that cannot be read or parsed is an error, unless the
// plan changes it (plannedDecls).
func declaredBy(r Repo, f string, planned map[string]plan.Change, decls declarations) (pkg string, idents []string, err error) {
	if c, ok := planned[f]; ok {
		pkg, idents = plannedDecls(decls, f, c.New)

		return pkg, idents, nil
	}

	src, err := fs.ReadFile(r.FS, f)
	if err != nil {
		return "", nil, fmt.Errorf("read %s: %w", f, err)
	}

	return decls(f, src)
}

// plannedDecls returns what decls reads from src, the content of the file at p before or after the
// plan changes it. A planned file that does not parse declares nothing here, on purpose: rule R5
// reports it in its turn, after R2 to R4, since a broken file most often follows from a place R4
// did not find.
func plannedDecls(decls declarations, p string, src []byte) (pkg string, idents []string) {
	pkg, idents, err := decls(p, src)
	if err != nil {
		return "", nil
	}

	return pkg, idents
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
// decls: every one of a created file, and those an edit declares more often than the file did. A
// change that leaves the file broken adds nothing here (plannedDecls).
func added(c plan.Change, decls declarations) (pkg string, adds []string) {
	pkg, after := plannedDecls(decls, c.Path, c.New)
	_, before := plannedDecls(decls, c.Path, c.Old)

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
// share one namespace, and the methods of its services, as Service.Method. The package is the
// folder, whose files are one proto package, so it is left empty.
func protoTopLevel(p string, src []byte) (pkg string, idents []string, err error) {
	if src == nil {
		return "", nil, nil
	}

	f, err := edit.ParseProto(p, src)
	if err != nil {
		return "", nil, err
	}

	return "", slices.Concat(f.Services(), f.Messages(), f.RPCs()), nil
}

// graphqlTopLevel returns the types the .graphqls file src defines and the fields it gives every
// type it defines or extends, as Type.field. A type's fields are unique across the files of a
// schema, which is its folder, so the package is left empty.
func graphqlTopLevel(p string, src []byte) (pkg string, idents []string, err error) {
	if src == nil {
		return "", nil, nil
	}

	q, err := edit.ParseGraphQL(p, src)
	if err != nil {
		return "", nil, err
	}

	return "", slices.Concat(q.Types(), q.Fields()), nil
}

// topLevel returns the package src declares and its top-level identifiers, methods as
// Type.Method, in source order. The blank identifier and init, which a package may declare more
// than once, are left out.
func topLevel(p string, src []byte) (pkg string, idents []string, err error) {
	if src == nil {
		return "", nil, nil
	}

	file, err := parser.ParseFile(token.NewFileSet(), p, src, parser.SkipObjectResolution)
	if err != nil {
		return "", nil, err
	}

	for _, d := range file.Decls {
		for _, id := range declNames(d) {
			if id != "_" && id != "init" {
				idents = append(idents, id)
			}
		}
	}

	return file.Name.Name, idents, nil
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
