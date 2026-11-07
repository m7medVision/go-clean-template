package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/m7medvision/go-clean-template/tools/gen/edit"
	"github.com/m7medvision/go-clean-template/tools/gen/names"
	"github.com/m7medvision/go-clean-template/tools/gen/rules"
	"github.com/m7medvision/go-clean-template/tools/gen/templates"
)

// The kinds of use case, each also the package its message goes in, and the set of templates it
// renders under templates/usecase.
const (
	_kindCommand = "command"
	_kindQuery   = "query"
)

// usecaseLayers are the layers of a new use case: its message and handler, then its operation on
// each transport. Each creates its own files, one per operation as every domain keeps them; three
// files gain a line, since what they list cannot be split: Register, the REST NewRoutes and the gRPC
// service.
func usecaseLayers() []layer {
	return []layer{
		{creates: []string{"internal/usecase/{{.Name.Pkg}}/*/*.tmpl"}, edits: editRegister},
		{transport: "rest", creates: []string{controllerTemplates("rest") + "/*.tmpl"}, edits: editNewRoutes},
		{transport: "grpc", creates: []string{controllerTemplates("grpc") + "/*.tmpl"}, edits: editService},
		{transport: "graphql", creates: []string{rules.SchemaDir() + "/*.tmpl", controllerTemplates("graphql") + "/*.tmpl"}},
	}
}

// editRegister registers the handler in the domain's Register, after the last of its kind.
func editRegister(pl *planner) {
	handle := "cqrs.HandleCommand"
	if pl.data.Usecase.Kind == _kindQuery {
		handle = "cqrs.HandleQuery"
	}

	editFile(pl, "internal/usecase/"+pl.data.Name.Pkg+"/register.go", edit.ParseGo, func(g *edit.Go) ([]edit.Insertion, error) {
		return one(g.AfterLastStmt("Register", edit.Call(handle), pl.text("internal/usecase/{{.Name.Pkg}}/register.go.Register")))
	})
}

// editNewRoutes registers the REST operation after the domain's last one.
func editNewRoutes(pl *planner) {
	controller := rules.ControllerDir("rest", pl.data.Name.Pkg) + "/controller.go"

	editFile(pl, controller, edit.ParseGo, func(g *edit.Go) ([]edit.Insertion, error) {
		return one(g.AfterLastStmt("NewRoutes", edit.Call("r.*"), pl.text(controllerTemplates("rest")+"/controller.go.NewRoutes")))
	})
}

// editService adds the rpc to the service the domain's controller serves, and its request and
// response messages at the end of the .proto file that declares the service.
func editService(pl *planner) {
	u := pl.data.Usecase
	edits := rules.ProtoDir + "/{{.Name.Snake}}.proto."

	editFile(pl, u.ProtoFile, edit.ParseProto,
		func(p *edit.Proto) ([]edit.Insertion, error) {
			return one(p.InService(u.Service, pl.text(edits+"service")))
		},
		func(p *edit.Proto) ([]edit.Insertion, error) {
			return []edit.Insertion{p.AtEnd(pl.text(edits + "end"))}, nil
		},
	)
}

// newUsecase plans one command or query on an existing domain and writes it, or prints it on a
// dry run.
func newUsecase(ctx context.Context, req *usecaseRequest, stdout io.Writer) error {
	r := rules.Open(req.root)

	pl, err := planUsecase(ctx, r, req)
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

// planUsecase plans the use case req asks for on the repository r. Planning needs the names and
// the domain, so it checks first R1 on both names, the use case's against the app's packages too,
// then R2 for a use case: the domain exists, it has each transport the use case gets (unless req
// names them, those rules.UsecaseTransports gives it) with what the generated code uses there, and
// the message is free. Then it checks R1 on the identifiers the plan adds, and last R2 on the REST
// operation's route and ID, which must be free too. What planning runs into is in the planner's
// errs, for the rules execute runs.
func planUsecase(ctx context.Context, r rules.Repo, req *usecaseRequest) (*planner, error) {
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
		if transports, err = rules.UsecaseTransports(r, &domain, req.kind); err != nil {
			return nil, err
		}
	}

	if err := rules.UsecaseTargets(r, &domain, req.kind, &n, transports); err != nil {
		return nil, err
	}

	u := &templates.Usecase{Name: n, Kind: req.kind, Method: req.method, Path: req.path}
	u.Service, u.ProtoFile = rules.GRPCService(&domain)
	route(u, &domain)

	pl, err := newPlanner(r, _usecaseSet+"/"+req.kind, "new-usecase "+domain.Kebab+" "+n.Kebab,
		&templates.Data{Name: domain, Transports: transports, Usecase: u})
	if err != nil {
		return nil, err
	}

	pl.run(usecaseLayers(), transports)

	if err := usecaseFree(r, pl); err != nil {
		return nil, err
	}

	return pl, nil
}

// usecaseFree checks what pl planned for a use case on the repository r: R1 on the identifiers the
// plan adds, a message's fields among them, then R2 on the REST operation's route and ID.
func usecaseFree(r rules.Repo, pl *planner) error {
	u := pl.data.Usecase

	if err := rules.IdentifiersFree(r, &u.Name, pl.plan); err != nil {
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
