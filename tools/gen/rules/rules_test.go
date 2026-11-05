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

// A controller folder that is not a domain's, such as the REST response package or GraphQL's
// paging, takes its name too: the domain's controllers would be written into it.
func TestDomainFreeLooksAtTablesProtoServicesGraphQLTypesAndControllerFolders(t *testing.T) {
	t.Parallel()

	fsys := fstest.MapFS{
		"internal/app/schema/widgets.hcl":                   {Data: []byte("table \"widgets\" {}\n")},
		"docs/proto/v1/thing.proto":                         {Data: []byte("syntax = \"proto3\";\n\nservice GadgetService {}\n")},
		"internal/controller/graphql/schema/thing.graphqls": {Data: []byte("type Gizmo { id: ID! }\n")},
		"internal/controller/restapi/v1/response/token.go":  {Data: []byte("package response\n")},
		"internal/controller/grpc/v1/health/health.go":      {Data: []byte("package health\n")},
		"internal/controller/graphql/paging/paging.go":      {Data: []byte("package paging\n")},
	}

	for kebab, owner := range map[string]string{
		"widget":   "internal/app/schema/widgets.hcl",
		"gadget":   "service GadgetService",
		"gizmo":    "type Gizmo",
		"response": "internal/controller/restapi/v1/response exists",
		"health":   "internal/controller/grpc/v1/health exists",
		"paging":   "internal/controller/graphql/paging exists",
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

// A command on a domain whose GraphQL schema has no Mutation, as exchange's has only Query, gets no
// GraphQL field by default: the field would have no type to go on. Asked for, it fails R4.
func TestUsecaseTransportsLeaveOutGraphQLWithoutTheRootType(t *testing.T) {
	t.Parallel()

	fsys := fixtureRoot()
	fsys["internal/controller/graphql/schema/task.graphqls"] = &fstest.MapFile{
		Data: []byte("type Task { id: ID! }\n\ntype Query { task: Task }\n\ntype Mutation { deleteTask: Boolean }\n"),
	}
	fsys["internal/controller/graphql/schema/exchange.graphqls"] = &fstest.MapFile{
		Data: []byte("type Conversion { rate: String! }\n\nextend type Query { exchangeHistory: [Conversion!]! }\n"),
	}
	r := repoOf(fsys)

	for _, tc := range []struct {
		domain, kind string
		want         []string
	}{
		{"task", "command", []string{"rest", "grpc", "graphql"}},
		{"task", "query", []string{"rest", "grpc", "graphql"}},
		{"exchange", "command", []string{"rest", "grpc"}},
		{"exchange", "query", []string{"rest", "grpc", "graphql"}},
		{"user", "query", []string{"rest", "grpc"}},
	} {
		assert.Equal(t, tc.want, rules.UsecaseTransports(r, name(t, tc.domain), tc.kind), tc.domain+" "+tc.kind)
	}
}

// restOperations is a REST package with two operations, registered the way every domain does, and
// a test file whose operations the router never serves.
func restOperations() fstest.MapFS {
	file := func(s string) *fstest.MapFile { return &fstest.MapFile{Data: []byte(s)} }
	op := func(id, method, path string) string {
		return "\thuma.Register(api, huma.Operation{\n\t\tOperationID: \"" + id + "\",\n\t\tMethod: " + method +
			",\n\t\tPath: \"" + path + "\",\n\t}, nil)\n"
	}

	return fstest.MapFS{
		"internal/controller/restapi/v1/task/task.go": file("package task\n\nfunc ops(api huma.API) {\n" +
			op("count-tasks", "http.MethodGet", "/v1/tasks/count") + op("get-task", `"GET"`, "/v1/tasks/{id}") + "}\n"),
		"internal/controller/restapi/v1/user/user.go":      file("package user\n\nfunc ops(api huma.API) {\n" + op("register", "http.MethodPost", "/v1/auth/register") + "}\n"),
		"internal/controller/restapi/v1/task/task_test.go": file("package task_test\n\nfunc ops(api huma.API) {\n" + op("seed-tasks", "http.MethodPost", "/v1/tasks/seed") + "}\n"),
	}
}

func TestRouteFreeRefusesARouteOrAnOperationIDTheAPIHas(t *testing.T) {
	t.Parallel()

	r := repoOf(restOperations())

	for what, tc := range map[string]struct {
		method, path, id string
		says             string
	}{
		"the same route": {
			"GET", "/v1/tasks/count", "count-done-tasks",
			"GET /v1/tasks/count is served already, by the operation count-tasks in internal/controller/restapi/v1/task/task.go: " +
				"give the use case its own path=",
		},
		"a parameter named otherwise": {"GET", "/v1/tasks/{taskId}", "get-task-status", "by the operation get-task"},
		"the same operation ID": {
			"POST", "/v1/tasks/{id}/register", "register",
			"internal/controller/restapi/v1/user/user.go registers the operation ID register already",
		},
	} {
		err := rules.RouteFree(r, tc.method, tc.path, tc.id)

		require.ErrorIs(t, err, rules.ErrTargets, what)
		assert.Contains(t, err.Error(), tc.says, what)
	}

	for what, tc := range map[string]struct{ method, path, id string }{
		"another method":         {"POST", "/v1/tasks/count", "recount-tasks"},
		"another path":           {"GET", "/v1/tasks/stats", "count-done-tasks"},
		"a route only tests use": {"POST", "/v1/tasks/seed", "seed-many-tasks"},
	} {
		require.NoError(t, rules.RouteFree(r, tc.method, tc.path, tc.id), what)
	}
}

// A message carries the caller as UserID and the record it acts on as <Domain>ID, so on user the
// two would be one field.
func TestRecordFieldFreeRefusesARecordIDNamedAsTheCallers(t *testing.T) {
	t.Parallel()

	err := rules.RecordFieldFree(name(t, "user"), "/v1/users/{id}/lock")
	require.ErrorIs(t, err, rules.ErrTargets)
	assert.Contains(t, err.Error(), "give it a path without {id}")

	require.NoError(t, rules.RecordFieldFree(name(t, "user"), "/v1/user/lock"), "acts on no record")
	require.NoError(t, rules.RecordFieldFree(name(t, "task"), "/v1/tasks/{id}/archive"), "carries TaskID")
}
