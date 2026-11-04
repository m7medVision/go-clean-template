package rules

import (
	"fmt"
	"slices"

	"github.com/m7medvision/go-clean-template/tools/gen/edit"
	"github.com/m7medvision/go-clean-template/tools/gen/names"
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
