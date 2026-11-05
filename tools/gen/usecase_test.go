package main

import (
	"bytes"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
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

	pl, err := planUsecase(rules.Open(dir), templates.FS(), req)
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

			files[_dryRunGolden] = dryRun.Bytes()
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
		pl := planOnWidget(t, dir, req)
		created := createdFiles(t, pl.plan)

		for tmpl := range pl.used {
			if strings.HasPrefix(tmpl, path.Join(_usecaseSet, _editsDir)+"/") {
				continue
			}

			f, err := templates.Render(templates.FS(), tmpl, pl.data)
			require.NoError(t, err)
			assert.Equal(t, string(created[f.Path]), string(f.Content),
				"%s does not render go/format-clean for %s: write it as the file gofmt would", tmpl, req.name)
		}
	}
}

func TestEveryUsecaseTemplateIsUsedByALayer(t *testing.T) {
	t.Parallel()

	dir := widgetRepo(t)
	used := map[string]bool{}

	for _, req := range goldenUsecases() {
		for tmpl := range planOnWidget(t, dir, req).used {
			used[tmpl] = true
		}
	}

	require.NoError(t, fs.WalkDir(templates.FS(), _usecaseSet, func(p string, d fs.DirEntry, err error) error {
		if err == nil && !d.IsDir() {
			assert.True(t, used[p], "no layer of new-usecase uses %s", p)
		}

		return err
	}))
}

// The hand-written domains name their read models, tags and test helpers their own way, and
// new-usecase plans both kinds on each of them against this repository's real files.
func TestPlanningAUsecaseOnEachDomainOfThisRepositoryPassesEveryRule(t *testing.T) {
	t.Parallel()

	for _, req := range []usecaseRequest{
		{domain: "task", name: "archive-task", kind: "command"},
		{domain: "task", name: "count-tasks", kind: "query"},
		{domain: "user", name: "lock-user", kind: "command"},
		{domain: "user", name: "count-users", kind: "query"},
		{domain: "exchange", name: "cancel-conversion", kind: "command"},
		{domain: "exchange", name: "count-conversions", kind: "query"},
	} {
		t.Run(req.domain+"/"+req.name, func(t *testing.T) {
			t.Parallel()

			r := thisRepo()

			pl, err := planUsecase(r, templates.FS(), &req)
			require.NoError(t, err)

			require.Empty(t, pl.errs)
			require.NoError(t, rules.Check(t.Context(), r, pl.plan, regenerated(), pl.errs))
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
		pl, err := planUsecase(thisRepo(), templates.FS(), &tc.req)
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

func TestNewUsecaseRefusesWhatARuleForbidsBeforePlanning(t *testing.T) {
	t.Parallel()

	for name, tc := range map[string]struct {
		req  usecaseRequest
		rule error
		says string
	}{
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
			usecaseRequest{domain: "task", name: "create-task", kind: "query"},
			names.ErrInvalid, "internal/controller/restapi/v1/task/task.go would declare",
		},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			req := tc.req
			req.root = filepath.Join("..", "..")
			req.dryRun = true // a test of a rule that wrongly passed must still write nothing here

			err := newUsecase(t.Context(), &req, io.Discard)

			require.ErrorIs(t, err, tc.rule)
			assert.Contains(t, err.Error(), tc.says)
		})
	}
}

// A hand-written domain may lack a place new-usecase edits. It fails R4, naming what it looked
// for, and writes nothing.
func TestNewUsecaseOnADomainWithoutAnAnchorFailsR4ByName(t *testing.T) {
	t.Parallel()

	const (
		controller = "internal/controller/restapi/v1/widget/controller.go"
		register   = "internal/usecase/widget/register.go"
	)

	for name, tc := range map[string]struct {
		file, old, with string
		kind            string
		says            string
	}{
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
			register + ": found a statement in Register that is a call to cqrs.HandleQuery 0 times",
		},
		"no read model in Deps": {
			register, "repo.WidgetReadModel", "WidgetReadModel", "query",
			register + ": found a field of Deps of type repo.*ReadModel 0 times",
		},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			dir := widgetRepo(t)
			replaceIn(t, dir, tc.file, tc.old, tc.with)
			commitAll(t, dir, "without the anchor")

			before := fingerprint(t, dir)

			err := newUsecase(t.Context(), &usecaseRequest{root: dir, domain: "widget", name: "archive-widget", kind: tc.kind}, io.Discard)

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
			assert.Contains(t, out.String(), "created 2, edited 4")

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
		require.NoError(t, rules.Check(t.Context(), rules.Open(dir), pl.plan, regenerated(), pl.errs))

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
