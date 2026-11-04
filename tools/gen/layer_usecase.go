package main

import (
	"github.com/m7medvision/go-clean-template/tools/gen/edit"
)

// planUseCase plans the commands and queries every domain gets, as stubs with their tests and the
// Register that puts them on the buses, and wires the domain into internal/app/buses.go: its
// Register call in registerHandlers, its projector in subscribers, and both imports, the
// repository's under the alias persist<Name>Repo, as the other domains' are.
func planUseCase(pl *planner) {
	pl.create("internal/usecase/{{.Name.Pkg}}/*.tmpl")
	pl.create("internal/usecase/{{.Name.Pkg}}/command/*.tmpl")
	pl.create("internal/usecase/{{.Name.Pkg}}/query/*.tmpl")

	n := pl.data.Name
	repoPath := pl.data.Module + "/internal/repo/persistent/" + n.Pkg
	usecasePath := pl.data.Module + "/internal/usecase/" + n.Pkg

	editFile(pl, "internal/app/buses.go", edit.ParseGo,
		func(g *edit.Go) ([]edit.Insertion, error) {
			return one(g.AfterLastStmt("registerHandlers", edit.Call("*.Register"),
				pl.text("internal/app/buses.go.registerHandlers")))
		},
		func(g *edit.Go) ([]edit.Insertion, error) {
			return one(g.AfterLastElement("subscribers", edit.Call("*.NewProjector"),
				pl.text("internal/app/buses.go.subscribers")))
		},
		func(g *edit.Go) ([]edit.Insertion, error) { return g.Import(repoPath, "persist"+n.Go+"Repo") },
		func(g *edit.Go) ([]edit.Insertion, error) { return g.Import(usecasePath, "") },
	)
}
