package rules_test

import (
	"context"
	"testing"
	"testing/fstest"

	"github.com/m7medvision/go-clean-template/tools/gen/names"
	"github.com/m7medvision/go-clean-template/tools/gen/rules"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fixtureRoot is a repository with three domains: task on every transport, exchange on every
// transport with its read model under another name, and user without GraphQL.
func fixtureRoot() fstest.MapFS {
	file := func(s string) *fstest.MapFile { return &fstest.MapFile{Data: []byte(s)} }

	return fstest.MapFS{
		"internal/entity/task.go":                             file("package entity\n"),
		"internal/usecase/task/command/create_task.go":        file("package command\n\ntype CreateTask struct{}\n"),
		"internal/usecase/task/query/list_tasks.go":           file("package query\n\ntype ListTasks struct{}\n\ntype GetTask struct{}\n"),
		"internal/usecase/exchange/register.go":               file("package exchange\n"),
		"internal/usecase/user/register.go":                   file("package user\n"),
		"internal/repo/persistent/conversion/conversion.go":   file("package conversion\n"),
		"internal/app/schema/tasks.hcl":                       file("table \"tasks\" {}\n"),
		"docs/proto/v1/task.proto":                            file("syntax = \"proto3\";\n\nservice TaskService {}\n"),
		"internal/controller/graphql/schema/task.graphqls":    file("type Task { id: ID! }\n"),
		"internal/controller/restapi/v1/task/controller.go":   file("package task\n"),
		"internal/controller/grpc/v1/task/controller.go":      file("package task\n"),
		"internal/controller/graphql/task/resolver.go":        file("package task\n"),
		"internal/controller/restapi/v1/exchange/exchange.go": file("package exchange\n"),
		"internal/controller/grpc/v1/exchange/exchange.go":    file("package exchange\n"),
		"internal/controller/graphql/exchange/query.go":       file("package exchange\n"),
		"internal/controller/restapi/v1/user/controller.go":   file("package user\n"),
		"internal/controller/grpc/v1/user/controller.go":      file("package user\n"),
	}
}

func repoOf(fsys fstest.MapFS) rules.Repo {
	return rules.Repo{FS: fsys, Dirty: func(context.Context, []string) ([]string, error) { return nil, nil }}
}

func name(t *testing.T, kebab string) *names.Name {
	t.Helper()

	n, err := names.Parse(kebab, "")
	require.NoError(t, err)

	return &n
}

func TestDomainFreeAcceptsANewName(t *testing.T) {
	t.Parallel()

	require.NoError(t, rules.DomainFree(repoOf(fixtureRoot()), name(t, "invoice")))
}

func TestDomainFreeRejectsAnythingAnExistingDomainOwns(t *testing.T) {
	t.Parallel()

	for kebab, owner := range map[string]string{
		"task":       "internal/usecase/task",
		"user":       "internal/usecase/user",
		"conversion": "internal/repo/persistent/conversion",
		"exchange":   "internal/usecase/exchange",
	} {
		err := rules.DomainFree(repoOf(fixtureRoot()), name(t, kebab))
		require.ErrorIs(t, err, names.ErrInvalid, kebab)
		assert.Contains(t, err.Error(), owner, kebab)
	}
}

func TestDomainFreeLooksAtTablesProtoServicesAndGraphQLTypes(t *testing.T) {
	t.Parallel()

	fsys := fstest.MapFS{
		"internal/app/schema/widgets.hcl":                   {Data: []byte("table \"widgets\" {}\n")},
		"docs/proto/v1/thing.proto":                         {Data: []byte("syntax = \"proto3\";\n\nservice GadgetService {}\n")},
		"internal/controller/graphql/schema/thing.graphqls": {Data: []byte("type Gizmo { id: ID! }\n")},
	}

	for kebab, owner := range map[string]string{
		"widget": "internal/app/schema/widgets.hcl",
		"gadget": "service GadgetService",
		"gizmo":  "type Gizmo",
	} {
		err := rules.DomainFree(repoOf(fsys), name(t, kebab))
		require.ErrorIs(t, err, names.ErrInvalid, kebab)
		assert.Contains(t, err.Error(), owner, kebab)
	}
}

func TestCreatableRefusesAFileThatExists(t *testing.T) {
	t.Parallel()

	r := repoOf(fixtureRoot())

	require.NoError(t, rules.Creatable(r, []string{"internal/entity/invoice.go"}))

	err := rules.Creatable(r, []string{"internal/entity/invoice.go", "internal/entity/task.go"})
	require.ErrorIs(t, err, rules.ErrTargets)
	assert.Contains(t, err.Error(), "internal/entity/task.go")
}

func TestReadOfAMissingFileToEditIsR2(t *testing.T) {
	t.Parallel()

	_, err := repoOf(fixtureRoot()).Read("internal/app/missing.go")
	require.ErrorIs(t, err, rules.ErrTargets)
}

func TestDomainTransportsListsTheFoldersADomainHas(t *testing.T) {
	t.Parallel()

	r := repoOf(fixtureRoot())

	assert.Equal(t, []string{"rest", "grpc", "graphql"}, rules.DomainTransports(r, "task"))
	assert.Equal(t, []string{"rest", "grpc", "graphql"}, rules.DomainTransports(r, "exchange"))
	assert.Equal(t, []string{"rest", "grpc"}, rules.DomainTransports(r, "user"))
}

func TestUsecaseTargetsCheckTheDomainTheMessageAndTheTransports(t *testing.T) {
	t.Parallel()

	r := repoOf(fixtureRoot())

	require.NoError(t, rules.UsecaseTargets(r, "task", "command", name(t, "archive-task"), []string{"rest", "graphql"}))

	for what, args := range map[string]struct {
		domain, kind, msg string
		transports        []string
	}{
		"no such domain":           {"invoice", "command", "archive-invoice", []string{"rest"}},
		"message taken":            {"task", "command", "create-task", []string{"rest"}},
		"declared in another file": {"task", "query", "get-task", nil},
		"transport it lacks":       {"user", "command", "lock-user", []string{"graphql"}},
	} {
		err := rules.UsecaseTargets(r, args.domain, args.kind, name(t, args.msg), args.transports)
		require.ErrorIs(t, err, rules.ErrTargets, what)
	}
}
