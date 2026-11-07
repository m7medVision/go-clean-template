package main

import (
	"bytes"
	"encoding/json"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/m7medvision/go-clean-template/tools/gen/edit"
	"github.com/m7medvision/go-clean-template/tools/gen/names"
	"github.com/m7medvision/go-clean-template/tools/gen/rules"
	"github.com/m7medvision/go-clean-template/tools/gen/templates"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// _usecaseGoldenDir holds, for each kind, what new-usecase plans for the widget domain:
// testdata/golden/usecase-command for archive-widget and usecase-query for count-widgets. Each
// created file is at its repository path plus .golden, and dry-run.golden is the dry run, which
// shows every insertion into the domain's files with the lines around it. UPDATE_GOLDEN=1
// rewrites them all.
const _usecaseGoldenDir = "testdata/golden/usecase"

// _dryRunGolden is the golden of a use case's dry run, beside its created files.
const _dryRunGolden = "dry-run"

// _nextGolden is the golden of the checklist a command prints after it writes, beside its created
// files: what to write next.
const _nextGolden = "next"

// goldenUsecases are the use cases with goldens, one of each kind, on the widget domain.
func goldenUsecases() map[string]*usecaseRequest {
	return map[string]*usecaseRequest{
		"command": {domain: "widget", name: "archive-widget", kind: "command"},
		"query":   {domain: "widget", name: "count-widgets", kind: "query"},
	}
}

// widgetRepo is a committed repository holding this repository's go.mod and shared files, with
// the widget domain that make new-domain writes into them on every transport: a generated domain,
// as new-usecase finds it.
func widgetRepo(t *testing.T) string {
	t.Helper()

	dir := sharedRepo(t)
	require.NoError(t, newDomain(t.Context(), &domainRequest{root: dir, name: "widget", transports: rules.Transports()}, io.Discard))
	commitAll(t, dir, "new-domain widget")

	return dir
}

// planOnWidget plans req, rooted at the repository dir, which holds the widget domain.
func planOnWidget(t *testing.T, dir string, req *usecaseRequest) *planner {
	t.Helper()

	req.root = dir

	pl, err := planUsecase(t.Context(), rules.Open(dir), req)
	require.NoError(t, err)
	require.Empty(t, pl.errs)

	return pl
}

func TestEveryUsecaseFileMatchesItsGolden(t *testing.T) {
	t.Parallel()

	dir := widgetRepo(t)

	for kind, req := range goldenUsecases() {
		t.Run(kind, func(t *testing.T) {
			t.Parallel()

			pl := planOnWidget(t, dir, req)
			goldenDir := _usecaseGoldenDir + "-" + kind

			var dryRun bytes.Buffer
			require.NoError(t, pl.plan.DryRun(&dryRun))

			files := createdFiles(t, pl.plan)
			require.Len(t, files, 2, "the message and its test")

			for p, content := range files {
				golden(t, filepath.Join(goldenDir, filepath.FromSlash(p)+".golden"), content)
			}

			golden(t, filepath.Join(goldenDir, _dryRunGolden+".golden"), dryRun.Bytes())
			golden(t, filepath.Join(goldenDir, _nextGolden+".golden"), []byte(pl.next))

			files[_dryRunGolden], files[_nextGolden] = dryRun.Bytes(), []byte(pl.next)
			staleGoldens(t, goldenDir, files)
		})
	}
}

// As the templates of new-domain do, those of new-usecase read as their files for widget, so
// formatting what they render changes nothing for it.
func TestEveryUsecaseTemplateReadsAsItsFileForWidget(t *testing.T) {
	t.Parallel()

	dir := widgetRepo(t)

	for _, req := range goldenUsecases() {
		readAsTheirFiles(t, planOnWidget(t, dir, req))
	}
}

func TestEveryUsecaseTemplateIsUsedByALayer(t *testing.T) {
	t.Parallel()

	dir := widgetRepo(t)

	var plans []*planner

	for _, req := range goldenUsecases() {
		plans = append(plans, planOnWidget(t, dir, req))
	}

	used := usedTemplates(t, plans...)

	for _, tmpl := range setTemplates(t, _usecaseSet) {
		assert.True(t, used[tmpl], "no layer of new-usecase uses %s", tmpl)
	}
}

// The hand-written domains name their read models, tags and test helpers their own way, and
// new-usecase plans both kinds on each of them against this repository's real files.
func TestPlanningAUsecaseOnEachDomainOfThisRepositoryPassesEveryRule(t *testing.T) {
	t.Parallel()

	for _, req := range []usecaseRequest{
		{domain: "task", name: "archive-task", kind: "command"},
		{domain: "task", name: "count-tasks", kind: "query"},
		{domain: "user", name: "lock-user", kind: "command", path: "/v1/user/lock"},
		{domain: "user", name: "count-users", kind: "query"},
		{domain: "exchange", name: "cancel-conversion", kind: "command"},
		{domain: "exchange", name: "count-conversions", kind: "query"},
	} {
		t.Run(req.domain+"/"+req.name, func(t *testing.T) {
			t.Parallel()

			r := thisRepo()

			pl, err := planUsecase(t.Context(), r, &req)
			require.NoError(t, err)

			require.Empty(t, pl.errs)
			require.NoError(t, rules.Check(t.Context(), r, pl.plan, pl.errs))
		})
	}
}

func TestPlanningAUsecaseReadsTheHandlersDependencyFromDeps(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		req           usecaseRequest
		dep, depType  string
		registerAfter string
	}{
		{usecaseRequest{domain: "task", name: "archive-task", kind: "command"}, "Store", "EventStore", "DeleteTaskHandler"},
		{usecaseRequest{domain: "task", name: "count-tasks", kind: "query"}, "Tasks", "TaskReadModel", "ListAllTasksHandler"},
		{usecaseRequest{domain: "exchange", name: "count-conversions", kind: "query"}, "History", "ExchangeReadModel", "GetConversionHandler"},
	} {
		pl, err := planUsecase(t.Context(), thisRepo(), &tc.req)
		require.NoError(t, err)

		u := pl.data.Usecase
		assert.Equal(t, tc.dep, u.Dep, tc.req.name)
		assert.Equal(t, tc.depType, u.DepType, tc.req.name)

		changes, err := pl.plan.Changes()
		require.NoError(t, err)

		for _, c := range changes {
			if strings.HasSuffix(c.Path, "/register.go") {
				assert.Contains(t, string(c.New), tc.registerAfter+"{"+tc.dep+": d."+tc.dep+"})\n\tcqrs.Handle",
					"%s: after the last of its kind", tc.req.name)
			}
		}
	}
}

// refusal is a use case that a rule refuses, and what the refusal says.
type refusal struct {
	req  usecaseRequest
	rule error
	says string
}

// refusals are the use cases on this repository's domains that a rule refuses before anything is
// planned or written, by what is wrong with each.
func refusals() map[string]refusal {
	return map[string]refusal{
		"a domain that does not exist": {
			usecaseRequest{domain: "invoice", name: "archive-invoice", kind: "command"},
			rules.ErrTargets, `there is no domain "invoice"`,
		},
		"a command name already taken": {
			usecaseRequest{domain: "task", name: "delete-task", kind: "command"},
			rules.ErrTargets, "internal/usecase/task/command already declares DeleteTask",
		},
		"a query name already taken": {
			usecaseRequest{domain: "task", name: "get-task", kind: "query"},
			rules.ErrTargets, "internal/usecase/task/query already declares GetTask",
		},
		"a transport the domain lacks": {
			usecaseRequest{domain: "user", name: "lock-user", kind: "command", transports: []string{"rest", "graphql"}},
			rules.ErrTargets, "user has no graphql controllers",
		},
		"a name that is not kebab-case": {
			usecaseRequest{domain: "task", name: "ArchiveTask", kind: "command"},
			names.ErrInvalid, "is not kebab-case",
		},
		"a name an initialism spells": {
			usecaseRequest{domain: "task", name: "export-task-json", kind: "query"},
			names.ErrInvalid, `the word "json"`,
		},
		"a name whose operation the domain has": {
			usecaseRequest{domain: "task", name: "create-task", kind: "query", transports: []string{"rest"}},
			names.ErrInvalid, "internal/controller/restapi/v1/task/task.go would declare",
		},
		"a name whose messages the proto has": {
			usecaseRequest{domain: "task", name: "create-task", kind: "query", transports: []string{"grpc"}},
			names.ErrInvalid, "docs/proto/v1/task.proto would declare CreateTaskRequest twice",
		},
		"a name whose field the schema has": {
			usecaseRequest{domain: "task", name: "all-tasks", kind: "query"},
			names.ErrInvalid, "internal/controller/graphql/schema/task.graphqls would declare Query.allTasks twice",
		},
		"a route the domain serves": {
			usecaseRequest{domain: "task", name: "get-task-status", kind: "query", path: "/v1/tasks/{id}"},
			rules.ErrTargets, "GET /v1/tasks/{id} is served already, by the operation get-task in " +
				"docs/openapi/v1.json: give the use case its own path=",
		},
		"an operation ID the API has": {
			usecaseRequest{domain: "task", name: "register", kind: "command", transports: []string{"rest"}},
			rules.ErrTargets, "docs/openapi/v1.json lists the operation ID register already, for POST /v1/auth/register",
		},
		"a name a package of the app has": {
			usecaseRequest{domain: "task", name: "command", kind: "command"},
			names.ErrInvalid, `"command" gives "command", the name of the package`,
		},
		"a record ID named as the caller's": {
			usecaseRequest{domain: "user", name: "lock-user", kind: "command"},
			rules.ErrTargets, "a use case that acts on one user, as {id} in its path /v1/users/{id}/lock says, " +
				"would carry its ID as UserID, which holds the caller's: give it a path without {id}",
		},
		"a GraphQL field on a schema without Mutation": {
			usecaseRequest{domain: "exchange", name: "cancel-conversion", kind: "command", transports: []string{"graphql"}},
			edit.ErrAnchor, "internal/controller/graphql/schema/exchange.graphqls: found type Mutation { … } 0 times",
		},
	}
}

// Each refusal comes from planning, or from the rules the plan then meets, against this repository
// with its working tree taken as clean: R3 reads the tree, and these tests must not depend on it.
func TestNewUsecaseRefusesWhatARuleForbidsBeforePlanning(t *testing.T) {
	t.Parallel()

	for name, tc := range refusals() {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			r, req := thisRepo(), tc.req

			pl, err := planUsecase(t.Context(), r, &req)
			if err == nil {
				err = rules.Check(t.Context(), r, pl.plan, pl.errs)
			}

			require.ErrorIs(t, err, tc.rule)
			assert.Contains(t, err.Error(), tc.says)
		})
	}
}

// missingAnchor is one place new-usecase edits, which a domain's file lacks: in file, old is
// replaced with with. Planning kind on that domain says says.
type missingAnchor struct {
	file, old, with string
	kind            string
	says            string
}

// missingAnchors are the places new-usecase edits, each taken out of the widget domain's files,
// by what is missing.
func missingAnchors() map[string]missingAnchor {
	const (
		controller     = "internal/controller/restapi/v1/widget/controller.go"
		register       = "internal/usecase/widget/register.go"
		grpcController = "internal/controller/grpc/v1/widget/controller.go"
		schema         = "internal/controller/graphql/schema/widget.graphqls"
	)

	return map[string]missingAnchor{
		"no problem method": {
			controller, "func (r *controller) problem(", "func (r *controller) explain(", "command",
			controller + ": found func problem 0 times",
		},
		"no NewRoutes": {
			controller, "func NewRoutes(", "func Routes(", "query",
			controller + ": found func NewRoutes 0 times",
		},
		"no tag constant": {
			controller, "const _widgetsTag", "const widgetsTag", "command",
			controller + ": found a constant _*Tag 0 times",
		},
		"no query handler in Register": {
			register, "\tcqrs.HandleQuery(", "\t// cqrs.HandleQuery(", "query",
			register + ": found a run of statements in Register that are each a call to cqrs.HandleQuery 0 times",
		},
		"no read model in Deps": {
			register, "repo.WidgetReadModel", "WidgetReadModel", "query",
			register + ": found a field of Deps of type repo.*ReadModel 0 times",
		},
		"no fail method in the gRPC controller": {
			grpcController, "func (c *Controller) fail(", "func (c *Controller) explain(", "command",
			grpcController + ": found func fail 0 times",
		},
		"no service the gRPC controller serves": {
			grpcController, "v1.UnimplementedWidgetServiceServer", "v1.WidgetServiceServer", "query",
			grpcController + ": found a field of Controller of type *.Unimplemented*ServiceServer 0 times",
		},
		"no proto file declaring the service": {
			"docs/proto/v1/widget.proto", "service WidgetService {", "service WidgetsService {", "command",
			"docs/proto/v1/*.proto: found service WidgetService 0 times",
		},
		"no Mutation in the schema": {
			schema, "extend type Mutation {", "extend type Subscription {", "command",
			schema + ": found type Mutation { … } 0 times",
		},
		"no Query in the schema": {
			schema, "extend type Query {", "extend type Subscription {", "query",
			schema + ": found type Query { … } 0 times",
		},
	}
}

// A hand-written domain may lack a place new-usecase edits. It fails R4, naming what it looked
// for, and writes nothing.
func TestNewUsecaseOnADomainWithoutAnAnchorFailsR4ByName(t *testing.T) {
	t.Parallel()

	for name, tc := range missingAnchors() {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			dir := widgetRepo(t)
			replaceIn(t, dir, tc.file, tc.old, tc.with)
			commitAll(t, dir, "without the anchor")

			before := fingerprint(t, dir)

			// Every transport, asked for: by default a use case gets no GraphQL field on a schema
			// without the root type it would go on.
			err := newUsecase(t.Context(), &usecaseRequest{
				root: dir, domain: "widget", name: "archive-widget", kind: tc.kind, transports: rules.Transports(),
			}, io.Discard)

			require.ErrorIs(t, err, edit.ErrAnchor)
			assert.Contains(t, err.Error(), tc.says)
			assert.Equal(t, before, fingerprint(t, dir))
		})
	}
}

// replaceIn replaces every occurrence of old, of which there is one at least, with replacement in
// the file at p in the repository at dir.
func replaceIn(t *testing.T, dir, p, old, replacement string) {
	t.Helper()

	root, err := os.OpenRoot(dir)
	require.NoError(t, err)

	defer root.Close()

	b, err := root.ReadFile(p)
	require.NoError(t, err)
	require.Contains(t, string(b), old, p)
	require.NoError(t, root.WriteFile(p, []byte(strings.ReplaceAll(string(b), old, replacement)), 0o600))
}

func TestNewUsecaseTwiceFailsR2TheSecondTimeAndWritesNothing(t *testing.T) {
	t.Parallel()

	for kind, req := range goldenUsecases() {
		t.Run(kind, func(t *testing.T) {
			t.Parallel()

			dir := widgetRepo(t)
			req.root = dir

			var out bytes.Buffer
			require.NoError(t, newUsecase(t.Context(), req, &out))
			assert.Contains(t, out.String(), "created 2, edited 10")
			assert.Contains(t, out.String(), "\nNext:\n", "the checklist, after the undo")

			register, err := os.ReadFile(filepath.Join(dir, "internal", "usecase", "widget", "register.go"))
			require.NoError(t, err)
			assert.Contains(t, string(register), map[string]string{
				"command": "DeleteWidgetHandler{Store: d.Store})\n\tcqrs.HandleCommand(cmds, command.ArchiveWidgetHandler{Store: d.Store})\n\n",
				"query":   "ListWidgetsHandler{Widgets: d.Widgets})\n\tcqrs.HandleQuery(qs, query.CountWidgetsHandler{Widgets: d.Widgets})\n}",
			}[kind], "after the last of its kind")

			commitAll(t, dir, "new-usecase "+req.name)
			before := fingerprint(t, dir)

			err = newUsecase(t.Context(), req, &bytes.Buffer{})
			require.ErrorIs(t, err, rules.ErrTargets)
			assert.Equal(t, before, fingerprint(t, dir))
		})
	}
}

// A path without {id} gives a command no record to act on, and one with it gives a query one:
// the message, its tests and the REST operation follow, and still pass every rule.
func TestAUsecaseActsOnARecordWhenItsPathNamesOne(t *testing.T) {
	t.Parallel()

	dir := widgetRepo(t)

	for _, tc := range []struct {
		req      usecaseRequest
		has, not []string
	}{
		{
			req: usecaseRequest{domain: "widget", name: "archive-widgets", kind: "command", method: "DELETE", path: "/v1/widgets/archived"},
			has: []string{"Method:      http.MethodDelete,", "_ *archiveWidgetsInput", "command.ArchiveWidgets{UserID: userID})"},
			not: []string{"WidgetID"},
		},
		{
			req: usecaseRequest{domain: "widget", name: "count-parts", kind: "query", path: "/v1/widgets/{id}/parts"},
			has: []string{"Path:        \"/v1/widgets/{id}/parts\",", "query.CountParts{UserID: userID, WidgetID: input.ID})", "WidgetID string"},
		},
	} {
		pl := planOnWidget(t, dir, &tc.req)
		require.NoError(t, rules.Check(t.Context(), rules.Open(dir), pl.plan, pl.errs))

		changes, err := pl.plan.Changes()
		require.NoError(t, err)

		var all strings.Builder
		for _, c := range changes {
			all.Write(c.New)
		}

		for _, want := range tc.has {
			assert.Contains(t, all.String(), want, tc.req.name)
		}

		for _, unwanted := range tc.not {
			for _, c := range changes {
				if c.Old == nil {
					assert.NotContains(t, string(c.New), unwanted, "%s: %s", tc.req.name, c.Path)
				}
			}
		}
	}
}

func TestRouteDefaultsFollowTheKindTheDomainsPluralAndTheFirstWord(t *testing.T) {
	t.Parallel()

	for name, tc := range map[string]struct {
		domain, usecase, kind string
		method, path          string
		wantMethod, wantPath  string
	}{
		"a command":           {"task", "archive-task", "command", "", "", "POST", "/v1/tasks/{id}/archive"},
		"a query":             {"task", "count-tasks", "query", "", "", "GET", "/v1/tasks/count"},
		"a two-word plural":   {"line-item", "split-line-item", "command", "", "", "POST", "/v1/line-items/{id}/split"},
		"a one-word use case": {"task", "archive", "command", "", "", "POST", "/v1/tasks/{id}/archive"},
		"a method given":      {"task", "archive-task", "command", "PUT", "", "PUT", "/v1/tasks/{id}/archive"},
		"a path given":        {"task", "count-tasks", "query", "", "/v1/tasks/stats", "GET", "/v1/tasks/stats"},
	} {
		u := &templates.Usecase{Name: *parseName(t, tc.usecase), Kind: tc.kind, Method: tc.method, Path: tc.path}

		route(u, parseName(t, tc.domain))

		assert.Equal(t, tc.wantMethod, u.Method, name)
		assert.Equal(t, tc.wantPath, u.Path, name)
	}
}

// Each transport's layer edits the domain's files on that transport. Hand-written domains name
// some of them their own way: user's gRPC controller serves AuthService, from auth.proto, and user
// and exchange have no gRPC test file, so they get one. Exchange's GraphQL resolves no mutations,
// so a command on it gets no GraphQL field unless it is asked for.
func TestAUsecaseEditsTheDomainsFilesOnEachOfItsTransports(t *testing.T) {
	t.Parallel()

	rest := func(domain string) []string {
		dir := "internal/controller/restapi/v1/" + domain + "/"

		return []string{dir + "controller.go", dir + domain + ".go", dir + domain + "_test.go", "internal/usecase/" + domain + "/register.go"}
	}

	for _, tc := range []struct {
		req            usecaseRequest
		edits, creates []string
	}{
		{
			req: usecaseRequest{domain: "task", name: "archive-task", kind: "command"},
			edits: append(rest("task"), "docs/proto/v1/task.proto",
				"internal/controller/grpc/v1/task/task.go", "internal/controller/grpc/v1/task/task_test.go",
				"internal/controller/graphql/schema/task.graphqls",
				"internal/controller/graphql/task/mutation.go", "internal/controller/graphql/task/mutation_test.go"),
		},
		{
			req: usecaseRequest{domain: "task", name: "count-tasks", kind: "query"},
			edits: append(rest("task"), "docs/proto/v1/task.proto",
				"internal/controller/grpc/v1/task/task.go", "internal/controller/grpc/v1/task/task_test.go",
				"internal/controller/graphql/schema/task.graphqls",
				"internal/controller/graphql/task/query.go", "internal/controller/graphql/task/query_test.go"),
		},
		{
			req:     usecaseRequest{domain: "user", name: "lock-user", kind: "command", path: "/v1/user/lock"},
			edits:   append(rest("user"), "docs/proto/v1/auth.proto", "internal/controller/grpc/v1/user/user.go"),
			creates: []string{"internal/controller/grpc/v1/user/user_test.go"},
		},
		{
			req:     usecaseRequest{domain: "exchange", name: "cancel-conversion", kind: "command"},
			edits:   append(rest("exchange"), "docs/proto/v1/exchange.proto", "internal/controller/grpc/v1/exchange/exchange.go"),
			creates: []string{"internal/controller/grpc/v1/exchange/exchange_test.go"},
		},
	} {
		pl, err := planUsecase(t.Context(), thisRepo(), &tc.req)
		require.NoError(t, err)
		require.Empty(t, pl.errs, tc.req.name)

		assert.ElementsMatch(t, tc.edits, pl.plan.Edited(), tc.req.name)

		created := createdFiles(t, pl.plan)
		for _, p := range tc.creates {
			require.Contains(t, created, p, tc.req.name)
			assert.Contains(t, string(created[p]), "package "+tc.req.domain+"_test\n", "%s: %s", tc.req.name, p)
			assert.Contains(t, string(created[p]), "func Test"+pl.data.Usecase.Name.Go+"RPC(t *testing.T) {", "%s: %s", tc.req.name, p)
		}
	}
}

// Two use cases whose names start with one word get one default path: count-widgets and
// count-done-widgets are both GET /v1/widgets/count. Once make openapi has exported the first, the
// second fails R2 before anything is written, naming the operation that serves the route, and
// passes with a path of its own.
func TestNewUsecaseOnARouteTheDomainServesFailsR2AndWritesNothing(t *testing.T) {
	t.Parallel()

	dir := widgetRepo(t)
	require.NoError(t, newUsecase(t.Context(), &usecaseRequest{root: dir, domain: "widget", name: "count-widgets", kind: "query"}, io.Discard))
	exportOperation(t, dir, "/v1/widgets/count", "get", "count-widgets")
	commitAll(t, dir, "new-usecase count-widgets")

	before := fingerprint(t, dir)

	err := newUsecase(t.Context(), &usecaseRequest{root: dir, domain: "widget", name: "count-done-widgets", kind: "query"}, io.Discard)

	require.ErrorIs(t, err, rules.ErrTargets)
	assert.Contains(t, err.Error(), "GET /v1/widgets/count is served already, by the operation count-widgets in "+
		"docs/openapi/v1.json: give the use case its own path=")
	assert.Equal(t, before, fingerprint(t, dir))

	require.NoError(t, newUsecase(t.Context(), &usecaseRequest{
		root: dir, domain: "widget", name: "count-done-widgets", kind: "query", path: "/v1/widgets/done-count", dryRun: true,
	}, io.Discard))
}

// exportOperation adds the operation id, served on method and route, to the OpenAPI document of
// the repository dir, as make openapi does once the operation is registered.
func exportOperation(t *testing.T, dir, route, method, id string) {
	t.Helper()

	p := filepath.Join(dir, "docs", "openapi", "v1.json")

	src, err := os.ReadFile(p)
	require.NoError(t, err)

	var doc map[string]any
	require.NoError(t, json.Unmarshal(src, &doc))

	paths, ok := doc["paths"].(map[string]any)
	require.True(t, ok, "the document has paths")

	if paths[route] == nil {
		paths[route] = map[string]any{}
	}

	item, ok := paths[route].(map[string]any)
	require.True(t, ok, "%s is a path item", route)

	item[method] = map[string]any{"operationId": id}

	out, err := json.MarshalIndent(doc, "", "  ")
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(p, out, 0o600))
}

// A query's GraphQL result type, <Name>Result, is a name the schema must not have yet.
func TestNewUsecaseRefusesAResultTypeTheSchemaHas(t *testing.T) {
	t.Parallel()

	const schema = "internal/controller/graphql/schema/widget.graphqls"

	dir := widgetRepo(t)
	replaceIn(t, dir, schema, "type WidgetList {", "type CountWidgetsResult { total: Int! }\n\ntype WidgetList {")
	commitAll(t, dir, "a type of that name")

	err := newUsecase(t.Context(), &usecaseRequest{root: dir, domain: "widget", name: "count-widgets", kind: "query", dryRun: true}, io.Discard)

	require.ErrorIs(t, err, names.ErrInvalid)
	assert.Contains(t, err.Error(), schema+" would declare CountWidgetsResult twice")
}

// The checklist printed after the write names the files to write next. Each one it names is
// created by the plan or is in the repository already.
func TestTheChecklistNamesFilesThatExist(t *testing.T) {
	t.Parallel()

	r := thisRepo()

	for _, req := range []usecaseRequest{
		{domain: "task", name: "archive-task", kind: "command"},
		{domain: "task", name: "count-tasks", kind: "query"},
		{domain: "exchange", name: "count-conversions", kind: "query"},
	} {
		pl, err := planUsecase(t.Context(), r, &req)
		require.NoError(t, err)

		checkNamedFiles(t, r, pl)
	}

	checkNamedFiles(t, r, planWidget(t))
}

// checkNamedFiles checks that every source file pl's checklist names is one pl creates or one in
// the repository r.
func checkNamedFiles(t *testing.T, r rules.Repo, pl *planner) {
	t.Helper()

	require.True(t, strings.HasPrefix(pl.next, "Next:\n"), pl.next)

	created := createdFiles(t, pl.plan)
	named := 0

	for _, word := range strings.Fields(pl.next) {
		word = strings.TrimRight(word, ",.;:)")
		if !slices.Contains([]string{".go", ".proto", ".graphqls", ".sql"}, path.Ext(word)) {
			continue
		}

		named++

		if _, ok := created[word]; !ok {
			_, err := fs.Stat(r.FS, word)
			assert.NoError(t, err, "%s names %s, which the plan does not create", pl.plan.Title(), word)
		}
	}

	assert.Positive(t, named, pl.plan.Title())
}
