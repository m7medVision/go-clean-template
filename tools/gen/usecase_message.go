package main

import (
	"strings"

	"github.com/m7medvision/go-clean-template/tools/gen/edit"
)

// planMessage plans the layer every use case gets: its message, a command or a query, with the
// handler stub and its test, in the domain's command or query package; and the handler's line in
// the domain's Register, after the last of its kind. The handler is built from the field of the
// domain's Deps that holds the event store, for a command, or the read model, for a query. A
// hand-written domain names that field as it likes (exchange's read model is History), so it is
// read from register.go, not derived from the domain's name.
func planMessage(pl *planner) {
	u := pl.data.Usecase
	register := "internal/usecase/" + pl.data.Name.Pkg + "/register.go"

	depType, handle := "repo.EventStore", "cqrs.HandleCommand"
	if u.Kind == _kindQuery {
		depType, handle = "repo.*ReadModel", "cqrs.HandleQuery"
	}

	editFile(pl, register, edit.ParseGo,
		func(g *edit.Go) ([]edit.Insertion, error) {
			dep, typ, err := g.FieldOfType("Deps", depType)
			u.Dep, u.DepType = dep, strings.TrimPrefix(typ, "repo.")

			return nil, err
		},
		func(g *edit.Go) ([]edit.Insertion, error) {
			return one(g.AfterLastStmt("Register", edit.Call(handle),
				pl.text("internal/usecase/{{.Name.Pkg}}/register.go.Register."+u.Kind)))
		},
	)

	pl.create("internal/usecase/{{.Name.Pkg}}/" + u.Kind + "/*.tmpl")
}
