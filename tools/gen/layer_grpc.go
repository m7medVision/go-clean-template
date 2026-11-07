package main

import (
	"github.com/m7medvision/go-clean-template/tools/gen/edit"
	"github.com/m7medvision/go-clean-template/tools/gen/rules"
)

// planGRPC plans the domain's gRPC service, for the grpc transport: docs/proto/v1/<name>.proto,
// whose <Name>Service has create, get, list, update and delete; the domain's folder in the gRPC
// controllers, which serves it, with its tests over bufconn; and the service's registration after
// the last one in the router's NewRoutes, with its import. make proto-v1, which make new-domain
// runs after the write, generates the service's Go code.
func planGRPC(pl *planner) {
	dir, tmpl := pl.controllers("grpc")

	pl.create(rules.ProtoDir + "/*.tmpl")
	pl.create(tmpl + "/*.tmpl")

	router := rules.ControllerRoot("grpc") + "/router.go"

	editFile(pl, router, edit.ParseGo,
		func(g *edit.Go) ([]edit.Insertion, error) {
			return one(g.AfterLastStmt("NewRoutes", edit.Call("v1.Register*ServiceServer"), pl.text(router+".NewRoutes")))
		},
		func(g *edit.Go) ([]edit.Insertion, error) { return g.Import(pl.importPath(dir), "") },
	)
}
