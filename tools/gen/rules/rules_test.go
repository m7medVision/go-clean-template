package rules_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"testing/fstest"

	"github.com/m7medvision/go-clean-template/tools/gen/names"
	"github.com/m7medvision/go-clean-template/tools/gen/plan"
	"github.com/m7medvision/go-clean-template/tools/gen/rules"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fixtureRoot is a repository with three domains: task on every transport, exchange on every
// transport with no GraphQL mutations, and user without GraphQL. Each declares what the code
// new-usecase generates uses: its Deps fields, its REST _tag and its GraphQL resolver types.
func fixtureRoot() fstest.MapFS {
	file := func(s string) *fstest.MapFile { return &fstest.MapFile{Data: []byte(s)} }

	return fstest.MapFS{
		"internal/entity/task.go":                             file("package entity\n"),
		"internal/usecase/task/register.go":                   file("package task\n\ntype Deps struct {\n\tStore int\n\tTasks int\n}\n"),
		"internal/usecase/task/command/create_task.go":        file("package command\n\ntype CreateTask struct{}\n"),
		"internal/usecase/task/query/list_tasks.go":           file("package query\n\ntype ListTasks struct{}\n\ntype GetTask struct{}\n"),
		"internal/usecase/exchange/register.go":               file("package exchange\n\ntype Deps struct{ Store, Exchanges int }\n"),
		"internal/usecase/user/register.go":                   file("package user\n\ntype Deps struct{ Store, Users int }\n"),
		"internal/repo/persistent/conversion/conversion.go":   file("package conversion\n"),
		"internal/app/migrations/00001_baseline.sql":          file("-- +goose Up\nCREATE TABLE tasks ();\n"),
		"docs/proto/v1/task.proto":                            file("syntax = \"proto3\";\n\nservice TaskService {}\n"),
		"internal/controller/graphql/schema/task.graphqls":    file("type Task { id: ID! }\n"),
		"internal/controller/restapi/v1/task/controller.go":   file("package task\n\nconst _tag = \"tasks\"\n"),
		"internal/controller/grpc/v1/task/controller.go":      file("package task\n"),
		"internal/controller/graphql/task/resolver.go":        file("package task\n\ntype Query struct{}\n\ntype Mutation struct{}\n"),
		"internal/controller/restapi/v1/exchange/exchange.go": file("package exchange\n\nconst _tag = \"exchange\"\n"),
		"internal/controller/grpc/v1/exchange/exchange.go":    file("package exchange\n"),
		"internal/controller/graphql/exchange/resolver.go":    file("package exchange\n\ntype Query struct{}\n"),
		"internal/controller/restapi/v1/user/controller.go":   file("package user\n\nconst _tag = \"users\"\n"),
		"internal/controller/grpc/v1/user/controller.go":      file("package user\n"),
	}
}

// repoOf is the repository with the files fsys, all clean, whose app has the packages
// appPackages lists.
func repoOf(fsys fstest.MapFS) rules.Repo {
	return rules.Repo{
		FS:       fsys,
		Status:   func(context.Context) (rules.Status, error) { return rules.Status{}, nil },
		Packages: func(context.Context) (map[string]string, error) { return appPackages(), nil },
	}
}

// appPackages are some packages of an app, by name, as go list names them.
func appPackages() map[string]string {
	return map[string]string{
		"command": "example.com/app/internal/usecase/task/command",
		"jwt":     "example.com/app/pkg/jwt",
		"v1":      "example.com/app/docs/proto/v1",
	}
}

func name(t *testing.T, kebab string) *names.Name {
	t.Helper()

	return named(t, kebab, "")
}

func named(t *testing.T, kebab, plural string) *names.Name {
	t.Helper()

	n, err := names.Parse(kebab, plural)
	require.NoError(t, err)

	return &n
}

func TestDomainFreeAcceptsANewName(t *testing.T) {
	t.Parallel()

	require.NoError(t, rules.DomainFree(t.Context(), repoOf(fixtureRoot()), name(t, "invoice")))
}

// A name the app's packages have would shadow the package in a generated file that imports it,
// whether the name or its plural gives it.
func TestPackageNameFreeRefusesAPackageOfTheApp(t *testing.T) {
	t.Parallel()

	r := repoOf(fixtureRoot())

	for _, n := range []*names.Name{name(t, "jwt"), name(t, "command"), named(t, "version", "v1")} {
		err := rules.PackageNameFree(t.Context(), r, n)
		require.ErrorIs(t, err, names.ErrInvalid, n.Kebab)
		assert.Contains(t, err.Error(), "the name of the package example.com/app/", n.Kebab)

		require.ErrorIs(t, rules.DomainFree(t.Context(), r, n), names.ErrInvalid, "%s: a domain", n.Kebab)
	}
}

// Open asks go list for the packages: those of the module rooted at the repository.
func TestOpenListsThePackagesOfTheModule(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "pkg", "jwt"), 0o750))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module example.com/app\n"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "pkg", "jwt", "jwt.go"), []byte("package jwt\n"), 0o600))

	err := rules.PackageNameFree(t.Context(), rules.Open(dir), name(t, "jwt"))

	require.ErrorIs(t, err, names.ErrInvalid)
	assert.Contains(t, err.Error(), "the name of the package example.com/app/pkg/jwt")
	require.NoError(t, rules.PackageNameFree(t.Context(), rules.Open(dir), name(t, "invoice")))
}

func TestDomainFreeRejectsAnythingAnExistingDomainOwns(t *testing.T) {
	t.Parallel()

	for kebab, owner := range map[string]string{
		"task":       "internal/usecase/task",
		"user":       "internal/usecase/user",
		"conversion": "internal/repo/persistent/conversion",
		"exchange":   "internal/usecase/exchange",
	} {
		err := rules.DomainFree(t.Context(), repoOf(fixtureRoot()), name(t, kebab))
		require.ErrorIs(t, err, names.ErrInvalid, kebab)
		assert.Contains(t, err.Error(), owner, kebab)
	}
}

// A controller folder that is not a domain's, such as the REST response package or GraphQL's
// paging, takes its name too: the domain's controllers would be written into it.
func TestDomainFreeLooksAtTablesProtoServicesGraphQLTypesAndControllerFolders(t *testing.T) {
	t.Parallel()

	fsys := fstest.MapFS{
		"internal/app/migrations/20260101000000_create_widgets.sql": {Data: []byte("-- +goose Up\n")},
		"docs/proto/v1/thing.proto":                                 {Data: []byte("syntax = \"proto3\";\n\nservice GadgetService {}\n")},
		"internal/controller/graphql/schema/thing.graphqls":         {Data: []byte("type Gizmo { id: ID! }\n")},
		"internal/controller/restapi/v1/response/token.go":          {Data: []byte("package response\n")},
		"internal/controller/grpc/v1/health/health.go":              {Data: []byte("package health\n")},
		"internal/controller/graphql/paging/paging.go":              {Data: []byte("package paging\n")},
	}

	for kebab, owner := range map[string]string{
		"widget":   "internal/app/migrations/20260101000000_create_widgets.sql exists",
		"gadget":   "service GadgetService",
		"gizmo":    "type Gizmo",
		"response": "internal/controller/restapi/v1/response exists",
		"health":   "internal/controller/grpc/v1/health exists",
		"paging":   "internal/controller/graphql/paging exists",
	} {
		err := rules.DomainFree(t.Context(), repoOf(fsys), name(t, kebab))
		require.ErrorIs(t, err, names.ErrInvalid, kebab)
		assert.Contains(t, err.Error(), owner, kebab)
	}
}

// A file the repository, or a domain's own files, cannot be read as cannot say what it declares,
// so the check fails on it rather than passing as if it declared nothing.
func TestDomainFreeFailsOnAFileThatDoesNotParse(t *testing.T) {
	t.Parallel()

	for _, p := range []string{"docs/proto/v1/broken.proto", "internal/controller/graphql/schema/broken.graphqls"} {
		fsys := fixtureRoot()
		fsys[p] = &fstest.MapFile{Data: []byte("service {")}

		err := rules.DomainFree(t.Context(), repoOf(fsys), name(t, "invoice"))

		require.Error(t, err, p)
		require.NotErrorIs(t, err, names.ErrInvalid, p)
		assert.Contains(t, err.Error(), p)
	}
}

func TestCreatableRefusesAFileThatExists(t *testing.T) {
	t.Parallel()

	r := repoOf(fixtureRoot())
	creating := func(paths ...string) *plan.Plan {
		p := plan.New("x")
		for _, path := range paths {
			p.Create(path, []byte("package entity\n"))
		}

		return p
	}

	require.NoError(t, rules.Check(t.Context(), r, creating("internal/entity/invoice.go"), nil))

	err := rules.Check(t.Context(), r, creating("internal/entity/invoice.go", "internal/entity/task.go"), nil)
	require.ErrorIs(t, err, rules.ErrTargets)
	assert.Contains(t, err.Error(), "internal/entity/task.go")
}

func TestReadOfAMissingFileToEditIsR2(t *testing.T) {
	t.Parallel()

	_, err := repoOf(fixtureRoot()).Read("internal/app/missing.go")
	require.ErrorIs(t, err, rules.ErrTargets)
}

func TestUsecaseTargetsCheckTheDomainTheMessageAndTheTransports(t *testing.T) {
	t.Parallel()

	r := repoOf(fixtureRoot())

	require.NoError(t, rules.UsecaseTargets(r, name(t, "task"), "command", name(t, "archive-task"), []string{"rest", "graphql"}))
	require.NoError(t, rules.UsecaseTargets(r, name(t, "exchange"), "query", name(t, "count-conversions"), rules.Transports()))

	for what, args := range map[string]struct {
		domain, kind, msg string
		transports        []string
		says              string
	}{
		"no such domain":           {"invoice", "command", "archive-invoice", []string{"rest"}, `there is no domain "invoice"`},
		"message taken":            {"task", "command", "create-task", []string{"rest"}, "already declares CreateTask"},
		"declared in another file": {"task", "query", "get-task", nil, "already declares GetTask"},
		"transport it lacks":       {"user", "command", "lock-user", []string{"graphql"}, "user has no graphql controllers"},
		"no resolver type":         {"exchange", "command", "cancel-conversion", []string{"graphql"}, "graphql/exchange declares no Mutation"},
	} {
		err := rules.UsecaseTargets(r, name(t, args.domain), args.kind, name(t, args.msg), args.transports)
		require.ErrorIs(t, err, rules.ErrTargets, what)
		assert.Contains(t, err.Error(), args.says, what)
	}

	fsys := fixtureRoot()
	fsys["internal/usecase/task/command/broken.go"] = &fstest.MapFile{Data: []byte("package command\n\nfunc {")}

	err := rules.UsecaseTargets(repoOf(fsys), name(t, "task"), "command", name(t, "archive-task"), nil)
	require.Error(t, err, "a message file that does not parse cannot say what it declares")
	assert.Contains(t, err.Error(), "internal/usecase/task/command/broken.go")
}

// The generated code uses what every domain declares, by the names every domain gives it; a domain
// that does not is refused before anything is planned.
func TestUsecaseTargetsNeedWhatTheGeneratedCodeUses(t *testing.T) {
	t.Parallel()

	for what, tc := range map[string]struct {
		file, src, kind string
		transports      []string
		says            string
	}{
		"the event store": {
			"internal/usecase/task/register.go", "package task\n\ntype Deps struct{ Tasks int }\n", "command", nil,
			"internal/usecase/task declares no Deps.Store",
		},
		"the read model": {
			"internal/usecase/task/register.go", "package task\n\ntype Deps struct{ Store, History int }\n", "query", nil,
			"internal/usecase/task declares no Deps.Tasks",
		},
		"the REST tag": {
			"internal/controller/restapi/v1/task/controller.go", "package task\n\nconst _tasksTag = \"tasks\"\n", "query",
			[]string{"rest"},
			"internal/controller/restapi/v1/task declares no _tag",
		},
	} {
		fsys := fixtureRoot()
		fsys[tc.file] = &fstest.MapFile{Data: []byte(tc.src)}

		err := rules.UsecaseTargets(repoOf(fsys), name(t, "task"), tc.kind, name(t, "count-tasks"), tc.transports)
		require.ErrorIs(t, err, rules.ErrTargets, what)
		assert.Contains(t, err.Error(), tc.says, what)
	}
}

// A command on a domain whose GraphQL resolvers have no Mutation, as exchange's have only Query,
// gets no GraphQL field by default: the field's resolver would be a method of no type. Asked for,
// it fails R2.
func TestUsecaseTransportsLeaveOutGraphQLWithoutTheRootType(t *testing.T) {
	t.Parallel()

	r := repoOf(fixtureRoot())

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
		got, err := rules.UsecaseTransports(r, name(t, tc.domain), tc.kind)
		require.NoError(t, err, tc.domain+" "+tc.kind)
		assert.Equal(t, tc.want, got, tc.domain+" "+tc.kind)
	}
}

// A GraphQL resolver file that does not parse cannot say which root types it resolves.
func TestUsecaseTransportsNeedTheResolversToParse(t *testing.T) {
	t.Parallel()

	const resolver = "internal/controller/graphql/exchange/resolver.go"

	fsys := fixtureRoot()
	fsys[resolver] = &fstest.MapFile{Data: []byte("package exchange\n\ntype Query struct {")}

	_, err := rules.UsecaseTransports(repoOf(fsys), name(t, "exchange"), "command")
	require.Error(t, err)
	assert.Contains(t, err.Error(), resolver)
}

// Every domain's gRPC service is named after it, but user's: AuthService, a name its clients use.
func TestGRPCServiceIsNamedAfterTheDomainButUsers(t *testing.T) {
	t.Parallel()

	for domain, want := range map[string][2]string{
		"task":      {"TaskService", "docs/proto/v1/task.proto"},
		"line-item": {"LineItemService", "docs/proto/v1/line_item.proto"},
		"user":      {"AuthService", "docs/proto/v1/auth.proto"},
	} {
		service, file := rules.GRPCService(name(t, domain))
		assert.Equal(t, want, [2]string{service, file}, domain)
	}
}

// restOperations is an OpenAPI document with three operations, one of them on a path item that
// has fields besides its operations.
func restOperations() fstest.MapFS {
	return fstest.MapFS{"docs/openapi/v1.json": {Data: []byte(`{
  "openapi": "3.1.0",
  "paths": {
    "/v1/tasks/count": {"get": {"operationId": "count-tasks"}},
    "/v1/tasks/{id}": {"summary": "One task", "parameters": [], "get": {"operationId": "get-task"}},
    "/v1/auth/register": {"post": {"operationId": "register"}}
  }
}
`)}}
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
			"GET /v1/tasks/count is served already, by the operation count-tasks in docs/openapi/v1.json: " +
				"give the use case its own path= (if the document is stale, run make openapi first)",
		},
		"a parameter named otherwise": {"GET", "/v1/tasks/{taskId}", "get-task-status", "by the operation get-task"},
		"the same operation ID": {
			"POST", "/v1/tasks/{id}/register", "register",
			"docs/openapi/v1.json lists the operation ID register already, for POST /v1/auth/register",
		},
	} {
		err := rules.RouteFree(r, tc.method, tc.path, tc.id)

		require.ErrorIs(t, err, rules.ErrTargets, what)
		assert.Contains(t, err.Error(), tc.says, what)
	}

	for what, tc := range map[string]struct{ method, path, id string }{
		"another method": {"POST", "/v1/tasks/count", "recount-tasks"},
		"another path":   {"GET", "/v1/tasks/stats", "count-done-tasks"},
	} {
		require.NoError(t, rules.RouteFree(r, tc.method, tc.path, tc.id), what)
	}
}

// The routes come from the OpenAPI document, so without one, or with one that is not JSON, the
// check cannot run: the error says to export it.
func TestRouteFreeNeedsTheOpenAPIDocument(t *testing.T) {
	t.Parallel()

	for what, fsys := range map[string]fstest.MapFS{
		"missing": {},
		"broken":  {"docs/openapi/v1.json": {Data: []byte(`{"paths": {"/v1/tasks": {"get": [`)}},
	} {
		err := rules.RouteFree(repoOf(fsys), "GET", "/v1/tasks/count", "count-tasks")

		require.Error(t, err, what)
		assert.Contains(t, err.Error(), "docs/openapi/v1.json", what)
		assert.Contains(t, err.Error(), "run make openapi", what)
	}
}
