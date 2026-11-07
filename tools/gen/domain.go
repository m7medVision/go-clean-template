package main

import (
	"context"
	"fmt"
	"go/token"
	"io"
	"os"

	"github.com/m7medvision/go-clean-template/tools/gen/edit"
	"github.com/m7medvision/go-clean-template/tools/gen/names"
	"github.com/m7medvision/go-clean-template/tools/gen/rules"
	"github.com/m7medvision/go-clean-template/tools/gen/templates"
)

// domainLayers are the layers of a new domain, from the entity out; a transport's layers name it,
// and run only when it was asked for. Every file they create copies the task file at its path, and
// three lists in the app gain the domain: the event types, the modules and, with GraphQL, the root
// resolver.
func domainLayers() []layer {
	return []layer{
		// The read shape and its sentinels, the aggregate, its events and the aggregate's test.
		{creates: []string{"internal/entity/*.tmpl"}, edits: editEventTypes},
		// The table, its queries, the read model's contract (which make mock mocks) and the read
		// model over them, and the projector that writes the table, with its test.
		{creates: []string{
			"internal/app/schema/*.tmpl", "internal/repo/persistent/queries/*.tmpl",
			"internal/repo/*.tmpl", "internal/repo/persistent/{{.Name.Pkg}}/*.tmpl",
		}},
		// The commands and queries, as stubs with their tests, and the Register that puts them on
		// the buses.
		{creates: []string{
			"internal/usecase/{{.Name.Pkg}}/*.tmpl",
			"internal/usecase/{{.Name.Pkg}}/command/*.tmpl", "internal/usecase/{{.Name.Pkg}}/query/*.tmpl",
		}},
		// The domain's module, which both binaries install: its handlers, projector and transports.
		{creates: []string{"internal/app/*.tmpl"}, edits: editModules},
		{transport: "rest", creates: []string{controllerTemplates("rest") + "/*.tmpl"}},
		// The service's .proto, which make proto-v1 compiles, and the controller that serves it.
		{transport: "grpc", creates: []string{rules.ProtoDir + "/*.tmpl", controllerTemplates("grpc") + "/*.tmpl"}},
		// The domain's types and one schema file per field, which make graphql compiles, and their
		// resolvers.
		{transport: "graphql", creates: []string{rules.SchemaDir() + "/*.tmpl", controllerTemplates("graphql") + "/*.tmpl"}, edits: editResolver},
		// The integration test, of every transport the domain has.
		{creates: []string{"integration-test/*.tmpl"}},
	}
}

// editEventTypes adds the domain's events to entity.EventTypes, so the store can decode them.
func editEventTypes(pl *planner) {
	editFile(pl, "internal/entity/event.go", edit.ParseGo, func(g *edit.Go) ([]edit.Insertion, error) {
		return one(g.AppendArg("EventTypes", "slices.Concat", pl.text("internal/entity/event.go.EventTypes")))
	})
}

// editModules adds the domain's module to the list both binaries install.
func editModules(pl *planner) {
	editFile(pl, "internal/app/modules.go", edit.ParseGo, func(g *edit.Go) ([]edit.Insertion, error) {
		return one(g.AppendElement("modules", "[]module", pl.text("internal/app/modules.go.modules")))
	})
}

// editResolver joins the domain's Query and Mutation to the root resolver: their aliases, their
// embeds, the statement in newResolver that fills them in, and the import.
func editResolver(pl *planner) {
	root := rules.ControllerRoot("graphql")
	resolver := root + "/resolver.go"

	editFile(pl, resolver, edit.ParseGo,
		func(g *edit.Go) ([]edit.Insertion, error) {
			return one(g.BeforeBlockEnd(token.TYPE, pl.text(resolver+".type")))
		},
		func(g *edit.Go) ([]edit.Insertion, error) {
			return one(g.AppendField("query", pl.text(resolver+".query")))
		},
		func(g *edit.Go) ([]edit.Insertion, error) {
			return one(g.AppendField("mutation", pl.text(resolver+".mutation")))
		},
		func(g *edit.Go) ([]edit.Insertion, error) {
			return one(g.AfterLastStmt("newResolver", edit.Call("*.New"), pl.text(resolver+".newResolver")))
		},
		func(g *edit.Go) ([]edit.Insertion, error) {
			return g.Import(pl.data.Module + "/" + rules.ControllerDir("graphql", pl.data.Name.Pkg))
		},
	)
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

	pl.run(domainLayers(), transports)

	return pl, nil
}
