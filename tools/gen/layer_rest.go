package main

import (
	"github.com/m7medvision/go-clean-template/tools/gen/edit"
	"github.com/m7medvision/go-clean-template/tools/gen/rules"
)

// planREST plans the domain's REST controllers, for the rest transport: the domain's folder in
// the REST controllers, which serves create, list, get, update and delete under /v1/<plural>, with
// its tests through fake handlers; and the domain's NewRoutes call after the last one in the
// router's NewRoutes, with its import.
func planREST(pl *planner) {
	dir, tmpl := pl.controllers("rest")
	pl.create(tmpl + "/*.tmpl")

	router := rules.ControllerRoot("rest") + "/router.go"

	editFile(pl, router, edit.ParseGo,
		func(g *edit.Go) ([]edit.Insertion, error) {
			return one(g.AfterLastStmt("NewRoutes", edit.Call("*.NewRoutes"), pl.text(router+".NewRoutes")))
		},
		func(g *edit.Go) ([]edit.Insertion, error) { return g.Import(pl.importPath(dir), "") },
	)
}
