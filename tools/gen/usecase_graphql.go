package main

import (
	"github.com/m7medvision/go-clean-template/tools/gen/edit"
	"github.com/m7medvision/go-clean-template/tools/gen/rules"
)

// planField plans the use case's GraphQL field, for the graphql transport: the field on the root
// type of the domain's schema file, schema/<name>.graphqls, Mutation for a command and Query for a
// query, written there as type or extend type; for a query, its result type, <Name>Result, at the
// end of that file, with one placeholder field, todo, since a GraphQL type cannot be empty; the
// resolver at the end of mutation.go or query.go in the domain's folder in the GraphQL
// controllers; and its test at the end of mutation_test.go or query_test.go there. A command's
// field answers the ID it wrote. The resolver maps errors with gqlerr.Map, as every resolver does.
// The test is self-contained, as the REST one is: it sends the field to the GraphQL handler with
// graphqltest, over buses holding only a fake handler for the use case's message. A domain without
// the test file gets one.
//
// A schema file without the root type fails R4, naming it, and nothing more is planned for the
// field: the domain has no resolver of that root type either, as exchange has no Mutation and no
// mutation.go, and the missing files would fail R2 first and hide why.
func planField(pl *planner) {
	u := pl.data.Usecase
	schema := rules.SchemaDir() + "/" + pl.data.Name.Snake + ".graphqls"
	tmpl := rules.SchemaDir() + "/{{.Name.Snake}}.graphqls"

	query := u.Kind == _kindQuery

	root, file := "Mutation", "mutation"
	if query {
		root, file = "Query", "query"
	}

	found := false
	fields := []func(*edit.GraphQL) ([]edit.Insertion, error){
		func(q *edit.GraphQL) ([]edit.Insertion, error) {
			ins, err := q.InType(root, pl.text(tmpl+"."+root))
			found = err == nil

			return one(ins, err)
		},
	}

	if query {
		fields = append(fields, func(q *edit.GraphQL) ([]edit.Insertion, error) {
			return []edit.Insertion{q.AtEnd(pl.text(tmpl + ".end.query"))}, nil
		})
	}

	editFile(pl, schema, edit.ParseGraphQL, fields...)

	if found {
		planResolver(pl, file)
	}
}

// planResolver plans the field's resolver at the end of file.go, mutation.go or query.go, in the
// domain's folder in the GraphQL controllers, with the imports it needs, and its test at the end of
// file_test.go there.
func planResolver(pl *planner, file string) {
	m := pl.data.Module
	dir, edits := pl.controllers("graphql")
	dir, edits = dir+"/", edits+"/"
	messages := m + "/internal/usecase/" + pl.data.Name.Pkg + "/" + pl.data.Usecase.Kind

	resolverImports := []string{
		"context", m + "/internal/controller/graphql/gqlerr", m + "/internal/controller/graphql/middleware",
		messages, m + "/pkg/cqrs",
	}
	if pl.data.Usecase.Kind == _kindQuery {
		resolverImports = append(resolverImports, m+"/internal/controller/graphql/model")
	}

	editFile(pl, dir+file+".go", edit.ParseGo, imports(
		func(g *edit.Go) ([]edit.Insertion, error) {
			return []edit.Insertion{g.AtEnd(pl.text(edits + file + ".go.end"))}, nil
		},
		resolverImports...,
	)...)

	test, text := dir+file+"_test.go", pl.text(edits+file+"_test.go.end")
	testImports := []string{
		"context", "testing", m + "/internal/controller/graphql/graphqltest", m + "/internal/entity", messages,
		m + "/pkg/cqrs", "github.com/stretchr/testify/assert", "github.com/stretchr/testify/require",
	}

	if !pl.repo.Exists(test) {
		pl.createTest(test, pl.data.Name.Pkg, testImports, text)

		return
	}

	editFile(pl, test, edit.ParseGo, imports(
		func(g *edit.Go) ([]edit.Insertion, error) { return []edit.Insertion{g.AtEnd(text)}, nil },
		testImports...,
	)...)
}
