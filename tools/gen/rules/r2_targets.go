package rules

import (
	"fmt"
	"path"
	"slices"
	"strings"

	"github.com/m7medvision/go-clean-template/tools/gen/edit"
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

// UsecaseTransports returns the transports a use case of kind (command or query) gets on domain
// when none are asked for: those it has a controller folder on, less GraphQL when the domain's
// schema file, schema/<name>.graphqls, neither defines nor extends the root type the use case's
// field goes on, Mutation for a command and Query for a query. Exchange resolves no mutations, so a
// command on it gets no GraphQL field unless it is asked for, and then fails R4, naming Mutation. A
// domain with GraphQL controllers and no schema file fails R2, which a file that does not parse
// would make the build fail.
func UsecaseTransports(r Repo, domain *names.Name, kind string) ([]string, error) {
	have := domainTransports(r, domain.Pkg)
	if !slices.Contains(have, "graphql") {
		return have, nil
	}

	root := "Query"
	if kind == "command" {
		root = "Mutation"
	}

	schema, err := parsed(r, SchemaDir()+"/"+domain.Snake+".graphqls", edit.ParseGraphQL)
	if err != nil {
		return nil, err
	}

	if schema.Declares(root) {
		return have, nil
	}

	return slices.DeleteFunc(have, func(t string) bool { return t == "graphql" }), nil
}

// RecordFieldFree checks R2 for a use case on domain whose REST path is route: when the path holds
// {id}, the use case acts on one record, and its message carries the record's ID as <Domain>ID
// beside the caller's UserID. On user the two would be one field, so a use case there acts on no
// record, which a path without {id} says, and its message gets the fields it needs by hand.
func RecordFieldFree(domain *names.Name, route string) error {
	if !strings.Contains(route, "{id}") || domain.Go+"ID" != "UserID" {
		return nil
	}

	return fmt.Errorf("%w: a use case that acts on one %s, as {id} in its path %s says, would carry its ID as UserID, "+
		"which holds the caller's: give it a path without {id} (path=), and its message the fields it needs",
		ErrTargets, domain.Words, route)
}

// UsecaseTargets checks R2 for a new use case: the domain exists, it has every one of
// transports, and no message of the kind (command or query) already has the use case's name.
func UsecaseTargets(r Repo, domain, kind string, n *names.Name, transports []string) error {
	dir := "internal/usecase/" + domain
	if !r.Exists(dir) {
		return fmt.Errorf("%w: there is no domain %q: %s does not exist", ErrTargets, domain, dir)
	}

	have := domainTransports(r, domain)
	for _, t := range transports {
		if !slices.Contains(have, t) {
			return fmt.Errorf("%w: %s has no %s controllers: %s does not exist", ErrTargets, domain, t, ControllerDir(t, domain))
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

	return nil
}

// declaresIn reports whether the Go package in the folder dir, which is named after it, declares
// name at its top level. Its external tests are another package and do not count.
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
