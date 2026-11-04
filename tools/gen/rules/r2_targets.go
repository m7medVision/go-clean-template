package rules

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"path"
	"slices"

	"github.com/m7medvision/go-clean-template/tools/gen/names"
)

// Transports are the transports a domain can have, in the order the generator lists them.
func Transports() []string {
	return []string{"rest", "grpc", "graphql"}
}

// transportDir is the folder a domain's controllers live in on transport t.
func transportDir(t, domain string) string {
	switch t {
	case "rest":
		return "internal/controller/restapi/v1/" + domain
	case "grpc":
		return "internal/controller/grpc/v1/" + domain
	default:
		return "internal/controller/graphql/" + domain
	}
}

// Creatable checks the first half of R2: none of the files a plan creates exists yet.
func Creatable(r Repo, paths []string) error {
	for _, p := range paths {
		if r.exists(p) {
			return fmt.Errorf("%w: %s, a file to create, exists", ErrTargets, p)
		}
	}

	return nil
}

// DomainTransports returns the transports domain has a controller folder on, in the order of
// Transports.
func DomainTransports(r Repo, domain string) []string {
	var have []string

	for _, t := range Transports() {
		if r.exists(transportDir(t, domain)) {
			have = append(have, t)
		}
	}

	return have
}

// UsecaseTargets checks R2 for a new use case: the domain exists, it has every one of
// transports, and no message of the kind (command or query) already has the use case's name.
func UsecaseTargets(r Repo, domain, kind string, n *names.Name, transports []string) error {
	dir := "internal/usecase/" + domain
	if !r.exists(dir) {
		return fmt.Errorf("%w: there is no domain %q: %s does not exist", ErrTargets, domain, dir)
	}

	have := DomainTransports(r, domain)
	for _, t := range transports {
		if !slices.Contains(have, t) {
			return fmt.Errorf("%w: %s has no %s controllers: %s does not exist", ErrTargets, domain, t, transportDir(t, domain))
		}
	}

	msgDir := path.Join(dir, kind)

	if declares(r, msgDir, n.Go) {
		return fmt.Errorf("%w: %s already declares %s", ErrTargets, msgDir, n.Go)
	}

	return nil
}

// declares reports whether a Go file in dir declares a type called name.
func declares(r Repo, dir, name string) bool {
	for _, p := range r.glob(dir + "/*.go") {
		src, err := r.Read(p)
		if err != nil {
			continue
		}

		f, err := parser.ParseFile(token.NewFileSet(), p, src, parser.SkipObjectResolution)
		if err != nil {
			continue
		}

		if declaresType(f, name) {
			return true
		}
	}

	return false
}

func declaresType(f *ast.File, name string) bool {
	for _, d := range f.Decls {
		gd, ok := d.(*ast.GenDecl)
		if !ok || gd.Tok != token.TYPE {
			continue
		}

		for _, s := range gd.Specs {
			if ts, ok := s.(*ast.TypeSpec); ok && ts.Name.Name == name {
				return true
			}
		}
	}

	return false
}
