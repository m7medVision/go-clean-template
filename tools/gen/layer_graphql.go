package main

import (
	"go/token"

	"github.com/m7medvision/go-clean-template/tools/gen/edit"
)

// planGraphQL plans the domain's GraphQL fields, for the graphql transport: the schema file
// internal/controller/graphql/schema/<name>.graphqls, which extends Query with a get and a list and
// Mutation with create, update and delete; the folder internal/controller/graphql/<pkg>, which
// resolves them, with its tests through fake handlers; and the shared files that join the domain
// in. The root resolver gets the domain's Query and Mutation aliases, their embeds and the New call
// that fills them, and the import; gqlerr.Map gets the domain's two sentinels, ahead of the
// entries any message shares; and gqlgen.yml binds the schema's type to the entity. make graphql,
// which make new-domain runs after the write, generates the rest.
func planGraphQL(pl *planner) {
	pl.create("internal/controller/graphql/schema/*.tmpl")
	pl.create("internal/controller/graphql/{{.Name.Pkg}}/*.tmpl")

	const (
		resolver = "internal/controller/graphql/resolver.go"
		gqlerr   = "internal/controller/graphql/gqlerr/gqlerr.go"
		gqlgen   = "internal/controller/graphql/gqlgen.yml"
	)

	resolverPath := pl.data.Module + "/internal/controller/graphql/" + pl.data.Name.Pkg

	editFile(pl, resolver, edit.ParseGo,
		func(g *edit.Go) ([]edit.Insertion, error) {
			return one(g.BeforeBlockEnd(token.TYPE, pl.text(resolver+".type")))
		},
		func(g *edit.Go) ([]edit.Insertion, error) {
			return one(g.AppendField("query", pl.text(resolver+".query")))
		},
		func(g *edit.Go) ([]edit.Insertion, error) {
			return one(g.AppendField("mutation", pl.text(resolver+".mutation")))
		},
		func(g *edit.Go) ([]edit.Insertion, error) {
			return one(g.AfterLastStmt("newResolver", edit.Call("*.New"), pl.text(resolver+".newResolver")))
		},
		func(g *edit.Go) ([]edit.Insertion, error) {
			return one(g.AppendElement("newResolver", "query", pl.text(resolver+".newResolver.query")))
		},
		func(g *edit.Go) ([]edit.Insertion, error) {
			return one(g.AppendElement("newResolver", "mutation", pl.text(resolver+".newResolver.mutation")))
		},
		func(g *edit.Go) ([]edit.Insertion, error) { return g.Import(resolverPath, "") },
	)

	editFile(pl, gqlerr, edit.ParseGo, func(g *edit.Go) ([]edit.Insertion, error) {
		return one(g.BeforeFirstElement("Map", edit.Leading("entity.ErrReadModelBehind"), pl.text(gqlerr+".Map")))
	})

	editFile(pl, gqlgen, edit.ParseYAML, func(y *edit.YAML) ([]edit.Insertion, error) {
		return one(y.InMap("models", pl.data.Name.Go, pl.text(gqlgen+".models")))
	})
}
