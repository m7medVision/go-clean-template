package rules

import (
	"fmt"
	"path"
	"slices"

	"github.com/m7medvision/go-clean-template/tools/gen/names"
)

// creatable checks the first half of R2: none of the files a plan creates exists yet.
func creatable(r Repo, paths []string) error {
	for _, p := range paths {
		if r.Exists(p) {
			return fmt.Errorf("%w: %s, a file to create, exists", ErrTargets, p)
		}
	}

	return nil
}

// domainTransports returns the transports domain has a controller folder on, in the order of
// Transports.
func domainTransports(r Repo, domain string) []string {
	var have []string

	for _, t := range Transports() {
		if r.Exists(ControllerDir(t, domain)) {
			have = append(have, t)
		}
	}

	return have
}

// resolverRoot is the root type a use case's GraphQL field goes on, whose resolver the domain's
// GraphQL package declares: Mutation for a command, Query for a query.
func resolverRoot(kind string) string {
	if kind == "command" {
		return "Mutation"
	}

	return "Query"
}

// UsecaseTransports returns the transports a use case of kind (command or query) gets on domain
// when none are asked for: those it has a controller folder on, less GraphQL when the domain's
// GraphQL package resolves no field of the root type the use case's field goes on (resolverRoot).
// Exchange resolves no mutations, so a command on it gets no GraphQL field unless it is asked for,
// and then fails R2, naming Mutation.
func UsecaseTransports(r Repo, domain *names.Name, kind string) ([]string, error) {
	have := domainTransports(r, domain.Pkg)
	if !slices.Contains(have, "graphql") {
		return have, nil
	}

	resolves, err := declaresIn(r, ControllerDir("graphql", domain.Pkg), resolverRoot(kind))
	if err != nil || resolves {
		return have, err
	}

	return slices.DeleteFunc(have, func(t string) bool { return t == "graphql" }), nil
}

// UsecaseTargets checks R2 for a new use case: the domain exists, it has every one of transports,
// and no message of the kind (command or query) already has the use case's name. The generated code
// relies on what every domain declares, so the domain must declare it too: the Deps field its
// handler is built from, Store (the event store) for a command and the domain's plural (its read
// model, as in Tasks) for a query; the _tag of its REST operations; and the GraphQL resolver type
// its field is a method of (resolverRoot).
func UsecaseTargets(r Repo, domain *names.Name, kind string, n *names.Name, transports []string) error {
	dir := "internal/usecase/" + domain.Pkg
	if !r.Exists(dir) {
		return fmt.Errorf("%w: there is no domain %q: %s does not exist", ErrTargets, domain.Kebab, dir)
	}

	have := domainTransports(r, domain.Pkg)
	for _, t := range transports {
		if !slices.Contains(have, t) {
			return fmt.Errorf("%w: %s has no %s controllers: %s does not exist", ErrTargets, domain.Pkg, t, ControllerDir(t, domain.Pkg))
		}
	}

	msgDir := path.Join(dir, kind)

	declared, err := declaresIn(r, msgDir, n.Go)
	if err != nil {
		return err
	}

	if declared {
		return fmt.Errorf("%w: %s already declares %s", ErrTargets, msgDir, n.Go)
	}

	return conventions(r, domain, kind, transports)
}

// conventions checks that domain declares what the code a use case of kind generates on transports
// uses by the names every domain gives it (UsecaseTargets).
func conventions(r Repo, domain *names.Name, kind string, transports []string) error {
	dir := "internal/usecase/" + domain.Pkg

	dep := "Deps.Store"
	if kind == "query" {
		dep = "Deps." + domain.Plural.Go
	}

	// Each folder, then the name it must declare.
	needs := [][2]string{{dir, dep}}

	if slices.Contains(transports, "rest") {
		needs = append(needs, [2]string{ControllerDir("rest", domain.Pkg), "_tag"})
	}

	if slices.Contains(transports, "graphql") {
		needs = append(needs, [2]string{ControllerDir("graphql", domain.Pkg), resolverRoot(kind)})
	}

	for _, need := range needs {
		if err := requireDecl(r, need[0], need[1]); err != nil {
			return err
		}
	}

	return nil
}

// requireDecl checks that the Go package in the folder dir declares name, which the generated code
// uses there.
func requireDecl(r Repo, dir, name string) error {
	declared, err := declaresIn(r, dir, name)
	if err != nil || declared {
		return err
	}

	return fmt.Errorf("%w: %s declares no %s, which the use case's code there uses as every domain declares it",
		ErrTargets, dir, name)
}

// declaresIn reports whether the Go package in the folder dir, which is named after it, declares
// name at its top level, a method or a struct field as Type.Name. Its external tests are another
// package and do not count.
func declaresIn(r Repo, dir, name string) (bool, error) {
	files, err := r.glob(dir + "/*.go")
	if err != nil {
		return false, err
	}

	for _, p := range files {
		src, err := r.Read(p)
		if err != nil {
			return false, err
		}

		pkg, idents, err := topLevel(p, src)
		if err != nil {
			return false, err
		}

		if pkg == path.Base(dir) && slices.Contains(idents, name) {
			return true, nil
		}
	}

	return false, nil
}
