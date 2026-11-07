package main

import (
	"context"
	"fmt"
	"go/format"
	"io"
	"io/fs"
	"os"
	"slices"
	"strconv"
	"strings"

	"github.com/m7medvision/go-clean-template/tools/gen/names"
	"github.com/m7medvision/go-clean-template/tools/gen/rules"
	"github.com/m7medvision/go-clean-template/tools/gen/templates"
)

// The kinds of use case, each also the package its message goes in.
const (
	_kindCommand = "command"
	_kindQuery   = "query"
)

// usecaseLayers are the layers of a new use case: its message and handler, then its operation on
// each transport. A layer is one file, usecase_<name>.go, and one entry here; a transport's layers
// name it, and run only when the use case gets it.
func usecaseLayers() []layer {
	return []layer{
		{plan: planMessage},
		{transport: "rest", plan: planOperation},
		{transport: "grpc", plan: planRPC},
		{transport: "graphql", plan: planField},
	}
}

// newUsecase plans one command or query on an existing domain and writes it, or prints it on a
// dry run.
func newUsecase(ctx context.Context, req *usecaseRequest, stdout io.Writer) error {
	r := rules.Open(req.root)

	pl, err := planUsecase(ctx, r, templates.FS(), req)
	if err != nil {
		return err
	}

	root, err := os.OpenRoot(req.root)
	if err != nil {
		return fmt.Errorf("open %s: %w", req.root, err)
	}
	defer root.Close()

	return execute(ctx, r, root, pl, req.dryRun, stdout)
}

// planUsecase plans the use case req asks for on the repository r, rendering the templates in
// tmpl. Planning needs the names and the domain, so it checks first R1 on both names, the use
// case's against the app's packages too, then R2 for a use case: the domain exists, it has each
// transport the use case gets (unless req names them, those rules.UsecaseTransports gives it), and
// the message is free. Then it checks R1 on the identifiers the plan adds, and last R2 on the
// message's fields and the REST operation's route and ID, which must be free too. What planning
// runs into is in the planner's errs, for the rules execute runs.
func planUsecase(ctx context.Context, r rules.Repo, tmpl fs.FS, req *usecaseRequest) (*planner, error) {
	n, err := names.Parse(req.name, "")
	if err != nil {
		return nil, err
	}

	if err := rules.PackageNameFree(ctx, r, &n); err != nil {
		return nil, err
	}

	domain, err := names.Parse(req.domain, req.plural)
	if err != nil {
		return nil, fmt.Errorf("the domain: %w", err)
	}

	transports := req.transports
	if transports == nil {
		transports = rules.UsecaseTransports(r, &domain, req.kind)
	}

	if err := rules.UsecaseTargets(r, domain.Pkg, req.kind, &n, transports); err != nil {
		return nil, err
	}

	u := &templates.Usecase{Name: n, Kind: req.kind, Method: req.method, Path: req.path}
	route(u, &domain)

	data := &templates.Data{Name: domain, Transports: transports, Usecase: u}
	pl := newPlanner(r, tmpl, _usecaseSet, "new-usecase "+domain.Kebab+" "+n.Kebab, data)

	module, err := modulePath(r)
	if err != nil {
		pl.fail(err)
	}

	data.Module = module

	for _, l := range selectLayers(usecaseLayers(), transports) {
		l.plan(pl)
	}

	pl.checklist()

	if err := usecaseFree(r, pl); err != nil {
		return nil, err
	}

	return pl, nil
}

// usecaseFree checks what pl planned for a use case on the repository r: R1 on the identifiers the
// plan adds, then R2 on the message's fields and, with REST, on the operation's route and ID.
func usecaseFree(r rules.Repo, pl *planner) error {
	u := pl.data.Usecase

	if err := rules.IdentifiersFree(r, &u.Name, pl.plan); err != nil {
		return err
	}

	if err := rules.RecordFieldFree(&pl.data.Name, u.Path); err != nil {
		return err
	}

	if pl.data.Has("rest") {
		return rules.RouteFree(r, u.Method, u.Path, u.Name.Kebab)
	}

	return nil
}

// route fills in the REST method and path u was not given: POST /v1/<plural>/{id}/<verb> for a
// command, which acts on one record, and GET /v1/<plural>/<verb> for a query, where the plural is
// the domain's and the verb is the first word of the use case's name: POST /v1/tasks/{id}/archive
// for archive-task, GET /v1/tasks/count for count-tasks.
func route(u *templates.Usecase, domain *names.Name) {
	verb, _, _ := strings.Cut(u.Name.Kebab, "-")
	base := "/v1/" + domain.Plural.Kebab + "/"

	if u.Method == "" {
		u.Method = "GET"
		if u.Kind == _kindCommand {
			u.Method = "POST"
		}
	}

	if u.Path == "" {
		u.Path = base + verb
		if u.Kind == _kindCommand {
			u.Path = base + "{id}/" + verb
		}
	}
}

// createTest plans the test file at p, which the domain does not have, in the external test package
// of pkg: the package clause, the imports specs name, each a path or "alias path" as an import line
// reads it, and text, the test a layer would append to the file if it had one. The imports are
// grouped as gci groups them, the standard library's first.
func (pl *planner) createTest(p, pkg string, specs []string, text string) {
	var std, others []string

	for _, spec := range specs {
		alias, importPath, ok := strings.Cut(spec, " ")
		if !ok {
			alias, importPath = "", spec
		}

		line := "\t" + strings.TrimSpace(alias+" "+strconv.Quote(importPath)) + "\n"
		if first, _, _ := strings.Cut(importPath, "/"); strings.Contains(first, ".") {
			others = append(others, line)
		} else {
			std = append(std, line)
		}
	}

	slices.SortFunc(others, func(a, b string) int { return strings.Compare(unaliased(a), unaliased(b)) })
	slices.Sort(std)

	content := "package " + pkg + "_test\n\nimport (\n" + strings.Join(std, "") + "\n" + strings.Join(others, "") + ")\n\n" + text + "\n"

	formatted, err := format.Source([]byte(content))
	if err == nil {
		content = string(formatted)
	}

	pl.plan.Create(p, []byte(content))
}

// unaliased is an import line's path, without its alias: what gci sorts the line by.
func unaliased(line string) string {
	return line[strings.IndexByte(line, '"'):]
}
