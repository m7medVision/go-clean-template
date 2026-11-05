package main

import (
	"go/token"

	"github.com/m7medvision/go-clean-template/tools/gen/edit"
)

// planEntity plans the entity layer, which every domain gets: the read shape, the aggregate, its
// events and the aggregate's test; the events in EventTypes, so the store can decode them; and the
// domain's two sentinel errors.
func planEntity(pl *planner) {
	pl.create("internal/entity/*.tmpl")

	editFile(pl, "internal/entity/event.go", edit.ParseGo, func(g *edit.Go) ([]edit.Insertion, error) {
		return one(g.AppendArg("EventTypes", "slices.Concat", pl.text("internal/entity/event.go.EventTypes")))
	})

	editFile(pl, "internal/entity/errors.go", edit.ParseGo, func(g *edit.Go) ([]edit.Insertion, error) {
		return one(g.BeforeBlockEnd(token.VAR, pl.text("internal/entity/errors.go.var")))
	})
}
