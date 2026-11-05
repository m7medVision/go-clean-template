package main

import (
	"github.com/m7medvision/go-clean-template/tools/gen/edit"
)

// planGRPC plans the domain's gRPC service, for the grpc transport: docs/proto/v1/<name>.proto,
// whose <Name>Service has create, get, list, update and delete; the folder
// internal/controller/grpc/v1/<pkg>, which serves it, with its tests over bufconn; and the
// service's registration after the last one in the v1 router's NewRoutes, with its import.
// make proto-v1, which make new-domain runs after the write, generates the service's Go code.
func planGRPC(pl *planner) {
	pl.create("docs/proto/v1/*.tmpl")
	pl.create("internal/controller/grpc/v1/{{.Name.Pkg}}/*.tmpl")

	const router = "internal/controller/grpc/v1/router.go"

	controllerPath := pl.data.Module + "/internal/controller/grpc/v1/" + pl.data.Name.Pkg

	editFile(pl, router, edit.ParseGo,
		func(g *edit.Go) ([]edit.Insertion, error) {
			return one(g.AfterLastStmt("NewRoutes", edit.Call("v1.Register*ServiceServer"), pl.text(router+".NewRoutes")))
		},
		func(g *edit.Go) ([]edit.Insertion, error) { return g.Import(controllerPath, "") },
	)
}
