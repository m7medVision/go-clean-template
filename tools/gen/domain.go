package main

import (
	"context"
	"fmt"
	"io"
	"os"

	"github.com/m7medvision/go-clean-template/tools/gen/names"
	"github.com/m7medvision/go-clean-template/tools/gen/rules"
	"github.com/m7medvision/go-clean-template/tools/gen/templates"
)

// domainLayers are the layers of a new domain, from the entity out. A layer is one file,
// layer_<name>.go, and one entry here; a transport's layers name it, and run only when it was
// asked for.
func domainLayers() []layer {
	return []layer{
		{plan: planEntity},
		{plan: planReadModel},
		{plan: planProjector},
		{plan: planUseCase},
		{transport: "rest", plan: planREST},
		{transport: "grpc", plan: planGRPC},
		{transport: "graphql", plan: planGraphQL},
		{plan: planIntegration},
	}
}

// newDomain plans the files of a whole domain and writes them, or prints them on a dry run.
func newDomain(ctx context.Context, req *domainRequest, stdout io.Writer) error {
	n, err := names.Parse(req.name, req.plural)
	if err != nil {
		return err
	}

	r := rules.Open(req.root)
	if err := rules.DomainFree(ctx, r, &n); err != nil {
		return err
	}

	pl, err := planDomain(r, &n, req.transports)
	if err != nil {
		return err
	}

	if err := rules.IdentifiersFree(r, &n, pl.plan); err != nil {
		return err
	}

	root, err := os.OpenRoot(req.root)
	if err != nil {
		return fmt.Errorf("open %s: %w", req.root, err)
	}
	defer root.Close()

	return execute(ctx, r, root, pl, req.dryRun, stdout)
}

// planDomain plans every layer of the domain n that runs for transports, against the repository r.
// What it runs into is in the planner's errs, for the rules.
func planDomain(r rules.Repo, n *names.Name, transports []string) (*planner, error) {
	pl, err := newPlanner(r, _domainSet, "new-domain "+n.Kebab, &templates.Data{Name: *n, Transports: transports})
	if err != nil {
		return nil, err
	}

	for _, l := range selectLayers(domainLayers(), transports) {
		l.plan(pl)
	}

	pl.checklist()

	return pl, nil
}
