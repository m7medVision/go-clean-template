package main

import (
	"github.com/m7medvision/go-clean-template/tools/gen/edit"
)

// planREST plans the domain's REST controllers, for the rest transport: the folder
// internal/controller/restapi/v1/<pkg>, which serves create, list, get, update and delete under
// /v1/<plural>, with its tests through fake handlers; and the domain's NewRoutes call after the
// last one in the v1 router's NewRoutes, with its import.
func planREST(pl *planner) {
	pl.create("internal/controller/restapi/v1/{{.Name.Pkg}}/*.tmpl")

	const router = "internal/controller/restapi/v1/router.go"

	controllerPath := pl.data.Module + "/internal/controller/restapi/v1/" + pl.data.Name.Pkg

	editFile(pl, router, edit.ParseGo,
		func(g *edit.Go) ([]edit.Insertion, error) {
			return one(g.AfterLastStmt("NewRoutes", edit.Call("*.NewRoutes"), pl.text(router+".NewRoutes")))
		},
		func(g *edit.Go) ([]edit.Insertion, error) { return g.Import(controllerPath, "") },
	)
}
