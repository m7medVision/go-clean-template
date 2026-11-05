package main

import (
	"go/token"

	"github.com/m7medvision/go-clean-template/tools/gen/edit"
)

// planReadModel plans the read side every domain gets: the table, as an Atlas HCL file; its sqlc
// queries; the read model over them, with its tracing decorator; and the read model's interface
// in contracts.go, which make mock then mocks.
func planReadModel(pl *planner) {
	pl.create("internal/app/schema/*.tmpl")
	pl.create("internal/repo/persistent/queries/*.tmpl")
	pl.create("internal/repo/persistent/{{.Name.Pkg}}/{{.Name.Snake}}.go.tmpl")
	pl.create("internal/repo/persistent/{{.Name.Pkg}}/tracing.go.tmpl")

	editFile(pl, "internal/repo/contracts.go", edit.ParseGo, func(g *edit.Go) ([]edit.Insertion, error) {
		return one(g.BeforeBlockEnd(token.TYPE, pl.text("internal/repo/contracts.go.type")))
	})
}
