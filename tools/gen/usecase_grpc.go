package main

import (
	"fmt"
	"io/fs"
	"slices"
	"strings"

	"github.com/m7medvision/go-clean-template/tools/gen/edit"
)

// _protoDir holds the .proto files, which are one proto package, grpc.v1.
const _protoDir = "docs/proto/v1"

// planRPC plans the use case's gRPC method, for the grpc transport: the rpc in the service the
// domain's controller serves, with its request and response messages at the end of the .proto file
// that declares the service; the method at the end of internal/controller/grpc/v1/<pkg>/<name>.go;
// and its test at the end of <name>_test.go there. The method maps errors with the controller's
// fail, as its other methods do. A hand-written domain's service need not be named after it
// (user's controller serves AuthService, from auth.proto), so the service is read from the
// Unimplemented<Service>Server its Controller embeds, and the .proto file is the one in
// docs/proto/v1 that declares it. The test is self-contained, as the REST one is: it serves the
// controller over bufconn, on buses holding only a fake handler for the use case's message, so it
// needs none of the folder's own test helpers. A domain without the test file, as user and
// exchange are, gets one.
func planRPC(pl *planner) {
	u := pl.data.Usecase
	dir := "internal/controller/grpc/v1/" + pl.data.Name.Pkg + "/"
	edits := "internal/controller/grpc/v1/{{.Name.Pkg}}/"
	m := pl.data.Module
	messages := m + "/internal/usecase/" + pl.data.Name.Pkg + "/" + u.Kind

	editFile(pl, dir+"controller.go", edit.ParseGo,
		func(g *edit.Go) ([]edit.Insertion, error) { return nil, g.RequireFunc("fail") },
		func(g *edit.Go) ([]edit.Insertion, error) {
			embed, err := g.FieldOfType("Controller", "*.Unimplemented*ServiceServer")
			u.GRPCService = strings.TrimSuffix(strings.TrimPrefix(embed.Name, "Unimplemented"), "Server")

			return nil, err
		},
	)

	if u.GRPCService != "" {
		planProto(pl)
	}

	editFile(pl, dir+pl.data.Name.Snake+".go", edit.ParseGo, imports(
		func(g *edit.Go) ([]edit.Insertion, error) {
			return []edit.Insertion{g.AtEnd(pl.text(edits + "{{.Name.Snake}}.go.end." + u.Kind))}, nil
		},
		"context", "v1 "+m+"/docs/proto/v1", "grpcmw "+m+"/internal/controller/grpc/middleware",
		messages, m+"/pkg/cqrs", "google.golang.org/grpc/codes", "google.golang.org/grpc/status",
	)...)

	planRPCTest(pl, dir+pl.data.Name.Snake+"_test.go", messages)
}

// planProto plans the rpc in the service the domain's controller serves, and its request and
// response messages at the end of the one .proto file in docs/proto/v1 that declares the service.
func planProto(pl *planner) {
	u := pl.data.Usecase
	edits := "docs/proto/v1/{{.Name.Snake}}.proto."

	files := protoFilesDeclaring(pl, u.GRPCService)
	if len(files) != 1 {
		pl.fail(fmt.Errorf("%w: %s/*.proto: found service %s %d times, want once", edit.ErrAnchor, _protoDir, u.GRPCService, len(files)))

		return
	}

	u.ProtoFile = files[0]

	editFile(pl, u.ProtoFile, edit.ParseProto,
		func(p *edit.Proto) ([]edit.Insertion, error) {
			return one(p.InService(u.GRPCService, pl.text(edits+"service")))
		},
		func(p *edit.Proto) ([]edit.Insertion, error) {
			return []edit.Insertion{p.AtEnd(pl.text(edits + "end." + u.Kind))}, nil
		},
	)
}

// protoFilesDeclaring returns the .proto files in docs/proto/v1 that declare the service called
// name. One that does not parse declares none here; make proto-v1 reports it.
func protoFilesDeclaring(pl *planner, name string) []string {
	var found []string

	files, err := fs.Glob(pl.repo.FS, _protoDir+"/*.proto")
	if err != nil {
		return nil
	}

	for _, p := range files {
		src, err := pl.repo.Read(p)
		if err != nil {
			continue
		}

		if proto, err := edit.ParseProto(p, src); err == nil && slices.Contains(proto.Services(), name) {
			found = append(found, p)
		}
	}

	return found
}

// planRPCTest plans the method's test at the end of the domain's gRPC test file at p, which
// imports the proto and the controller packages under names of its choosing, as task's imports the
// controller as task and a generated domain's as grpcwidget: the test calls them by those names.
// A domain without the file gets one, importing them as pb and under the controller's own name.
func planRPCTest(pl *planner, p, messages string) {
	u := pl.data.Usecase
	m := pl.data.Module
	proto, controller := m+"/docs/proto/v1", m+"/internal/controller/grpc/v1/"+pl.data.Name.Pkg
	text := "internal/controller/grpc/v1/{{.Name.Pkg}}/{{.Name.Snake}}_test.go.end." + u.Kind
	specs := []string{
		"context", "errors", "net", "testing", "time",
		"grpcmw " + m + "/internal/controller/grpc/middleware", m + "/internal/entity", messages,
		m + "/pkg/cqrs", m + "/pkg/jwt", m + "/pkg/logger",
		"github.com/stretchr/testify/assert", "github.com/stretchr/testify/require",
		"google.golang.org/grpc", "google.golang.org/grpc/codes", "google.golang.org/grpc/credentials/insecure",
		"google.golang.org/grpc/metadata", "google.golang.org/grpc/status", "google.golang.org/grpc/test/bufconn",
	}

	if !pl.exists(p) {
		u.ProtoPkg, u.ControllerPkg = "pb", pl.data.Name.Pkg
		pl.createTest(p, pl.data.Name.Pkg, append(specs, "pb "+proto, controller), pl.text(text))

		return
	}

	edits := []func(*edit.Go) ([]edit.Insertion, error){
		func(g *edit.Go) ([]edit.Insertion, error) {
			name, ins, err := g.ImportName(proto, "pb")
			u.ProtoPkg = name

			return ins, err
		},
		func(g *edit.Go) ([]edit.Insertion, error) {
			name, ins, err := g.ImportName(controller, "")
			u.ControllerPkg = name

			return ins, err
		},
	}

	editFile(pl, p, edit.ParseGo, append(edits, imports(
		func(g *edit.Go) ([]edit.Insertion, error) { return []edit.Insertion{g.AtEnd(pl.text(text))}, nil },
		specs...,
	)...)...)
}
