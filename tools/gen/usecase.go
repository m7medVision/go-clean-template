package main

import (
	"context"
	"fmt"
	"io"
	"io/fs"
	"os"
	"strings"

	"github.com/m7medvision/go-clean-template/tools/gen/names"
	"github.com/m7medvision/go-clean-template/tools/gen/rules"
	"github.com/m7medvision/go-clean-template/tools/gen/templates"
)

// usecaseLayers are the layers of a new use case: its message and handler, then its operation on
// each transport. A layer is one file, usecase_<name>.go, and one entry here; a transport's layers
// name it, and run only when the use case gets it.
func usecaseLayers() []layer {
	return []layer{
		{plan: planMessage},
		{transport: "rest", plan: planOperation},
	}
}

// newUsecase plans one command or query on an existing domain and writes it, or prints it on a
// dry run.
func newUsecase(ctx context.Context, req *usecaseRequest, stdout io.Writer) error {
	r := rules.Open(req.root)

	pl, err := planUsecase(r, templates.FS(), req)
	if err != nil {
		return err
	}

	root, err := os.OpenRoot(req.root)
	if err != nil {
		return fmt.Errorf("open %s: %w", req.root, err)
	}
	defer root.Close()

	return execute(ctx, r, root, pl.plan, pl.errs, req.dryRun, stdout)
}

// planUsecase plans the use case req asks for on the repository r, rendering the templates in
// tmpl. Planning needs the names and the domain, so it checks first R1 on both names, then R2 for
// a use case: the domain exists, it has each transport the use case gets (the domain's own unless
// req names them), and the message is free. Last it checks R1 on the identifiers the plan adds.
// What planning runs into is in the planner's errs, for the rules execute runs.
func planUsecase(r rules.Repo, tmpl fs.FS, req *usecaseRequest) (*planner, error) {
	n, err := names.Parse(req.name, "")
	if err != nil {
		return nil, err
	}

	domain, err := names.Parse(req.domain, req.plural)
	if err != nil {
		return nil, fmt.Errorf("the domain: %w", err)
	}

	transports := req.transports
	if transports == nil {
		transports = rules.DomainTransports(r, domain.Pkg)
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

	if err := rules.IdentifiersFree(r, &n, pl.plan); err != nil {
		return nil, err
	}

	return pl, nil
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
		if u.Kind == "command" {
			u.Method = "POST"
		}
	}

	if u.Path == "" {
		u.Path = base + verb
		if u.Kind == "command" {
			u.Path = base + "{id}/" + verb
		}
	}
}
