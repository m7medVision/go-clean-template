package main

import (
	"strings"

	"github.com/m7medvision/go-clean-template/tools/gen/edit"
)

// planOperation plans the use case's REST operation, for the rest transport, in the domain's
// folder in the REST controllers: the operation and its input and output structs at the end of
// <name>.go, its registration after the last one in NewRoutes, and its test at the end of
// <name>_test.go. The operation maps errors with the folder's problem method, and is tagged with
// the constant its other operations are, read from controller.go, since a hand-written domain
// names it as it likes (exchange's is _exchangeTag). The test is self-contained: it builds buses
// holding only a fake handler for the use case's message and serves them with resttest, so it
// needs nothing from the folder's own test helpers, which every domain names its own way.
func planOperation(pl *planner) {
	u := pl.data.Usecase
	dir, edits := pl.controllers("rest")
	dir, edits = dir+"/", edits+"/"
	messages := pl.data.Module + "/internal/usecase/" + pl.data.Name.Pkg + "/" + u.Kind

	editFile(pl, dir+"controller.go", edit.ParseGo,
		func(g *edit.Go) ([]edit.Insertion, error) { return nil, g.RequireFunc("problem") },
		func(g *edit.Go) ([]edit.Insertion, error) {
			tag, err := g.Const("_*Tag")
			u.RESTTag = tag

			return nil, err
		},
		func(g *edit.Go) ([]edit.Insertion, error) {
			return one(g.AfterLastStmt("NewRoutes", edit.Call("r.*"), pl.text(edits+"controller.go.NewRoutes")))
		},
	)

	editFile(pl, dir+pl.data.Name.Snake+".go", edit.ParseGo, imports(
		func(g *edit.Go) ([]edit.Insertion, error) {
			return []edit.Insertion{g.AtEnd(pl.text(edits + "{{.Name.Snake}}.go.end." + u.Kind))}, nil
		},
		"context", "net/http", "github.com/danielgtaylor/huma/v2",
		pl.data.Module+"/internal/controller/restapi/middleware",
		pl.data.Module+"/internal/controller/restapi/v1/apidoc",
		messages, pl.data.Module+"/pkg/cqrs",
	)...)

	editFile(pl, dir+pl.data.Name.Snake+"_test.go", edit.ParseGo, imports(
		func(g *edit.Go) ([]edit.Insertion, error) {
			return []edit.Insertion{g.AtEnd(pl.text(edits + "{{.Name.Snake}}_test.go.end." + u.Kind))}, nil
		},
		"context", "net/http", "testing",
		pl.data.Module+"/internal/controller/restapi/resttest",
		pl.data.Module+"/internal/entity",
		messages, pl.data.Module+"/pkg/cqrs",
		"github.com/stretchr/testify/assert", "github.com/stretchr/testify/require",
	)...)
}

// imports returns edits followed by one edit for each of specs, each a path or "alias path" as an
// import line reads it, which imports it unless the file does already: the imports what edits
// inserts needs.
func imports(edits func(*edit.Go) ([]edit.Insertion, error), specs ...string) []func(*edit.Go) ([]edit.Insertion, error) {
	all := []func(*edit.Go) ([]edit.Insertion, error){edits}

	for _, spec := range specs {
		alias, p, ok := strings.Cut(spec, " ")
		if !ok {
			alias, p = "", spec
		}

		all = append(all, func(g *edit.Go) ([]edit.Insertion, error) { return g.Import(p, alias) })
	}

	return all
}
