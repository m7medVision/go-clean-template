package main

import (
	"bytes"
	"context"
	"errors"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/m7medvision/go-clean-template/tools/gen/names"
	"github.com/m7medvision/go-clean-template/tools/gen/plan"
	"github.com/m7medvision/go-clean-template/tools/gen/rules"
	"github.com/m7medvision/go-clean-template/tools/gen/templates"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// _goldenDir holds what new-domain creates for widget on every transport, each file at its
// repository path plus .golden. A domain on only some transports has its goldens beside it, in
// _goldenDir-<transports>, the transports joined by dashes in the order rules.Transports lists
// them, as testdata/golden/domain-rest and domain-grpc-graphql (someTransports). Each holds only
// the files whose content differs from those in _goldenDir, such as the integration test, which
// tests only the transports asked for; a file the same either way is checked once, in _goldenDir.
// UPDATE_GOLDEN=1 rewrites them all.
const _goldenDir = "testdata/golden/domain"

// someTransports are the sets of transports, short of every one, that have goldens of their own.
func someTransports() [][]string {
	return [][]string{{"rest"}, {"grpc", "graphql"}}
}

// thisRepo is the repository this module sits in, read from disk, with every path clean: rule R3
// reads the working tree, and these tests must not depend on it.
func thisRepo() rules.Repo {
	r := rules.Open(filepath.Join("..", ".."))
	r.Dirty = func(context.Context, []string) ([]string, error) { return nil, nil }

	return r
}

func parseName(t *testing.T, kebab string) *names.Name {
	t.Helper()

	n, err := names.Parse(kebab, "")
	require.NoError(t, err)

	return &n
}

// planWidget plans widget on every transport against this repository's real files.
func planWidget(t *testing.T) *planner {
	t.Helper()

	return planDomain(thisRepo(), templates.FS(), parseName(t, "widget"), rules.Transports())
}

func createdFiles(t *testing.T, p *plan.Plan) map[string][]byte {
	t.Helper()

	changes, err := p.Changes()
	require.NoError(t, err)

	created := map[string][]byte{}

	for _, c := range changes {
		if c.Old == nil {
			created[c.Path] = c.New
		}
	}

	return created
}

func TestPlanningWidgetAgainstThisRepositoryPassesEveryRule(t *testing.T) {
	t.Parallel()

	r := thisRepo()
	require.NoError(t, rules.DomainFree(r, parseName(t, "widget")))

	pl := planWidget(t)

	require.Empty(t, pl.errs)
	require.NoError(t, rules.IdentifiersFree(r, parseName(t, "widget"), pl.plan))
	require.NoError(t, rules.Check(t.Context(), r, pl.plan, regenerated(), pl.errs))
}

// A name can own no file and still clash with what a package declares: record would declare the
// type entity.Record, and the entity package has a function called Record.
func TestNewDomainRefusesANameWhoseIdentifiersAPackageHas(t *testing.T) {
	t.Parallel()

	r := thisRepo()
	n := parseName(t, "record")
	require.NoError(t, rules.DomainFree(r, n), "no file or folder is called record")

	err := rules.IdentifiersFree(r, n, planDomain(r, templates.FS(), n, rules.Transports()).plan)

	require.ErrorIs(t, err, names.ErrInvalid)
	assert.Contains(t, err.Error(), "internal/entity/record.go would declare Record, which internal/entity/aggregate.go")
}

// Every .proto in docs/proto/v1 is the proto package grpc.v1, so a name free of files can still
// clash there: login would declare the message LoginResponse, which auth.proto declares. Without
// gRPC there is no login.proto, and nothing clashes.
func TestNewDomainRefusesANameWhoseProtoNamesAnotherFileDeclares(t *testing.T) {
	t.Parallel()

	r := thisRepo()
	n := parseName(t, "login")
	require.NoError(t, rules.DomainFree(r, n), "no file, folder or service is called login")

	err := rules.IdentifiersFree(r, n, planDomain(r, templates.FS(), n, rules.Transports()).plan)

	require.ErrorIs(t, err, names.ErrInvalid)
	assert.Contains(t, err.Error(), "docs/proto/v1/login.proto would declare LoginResponse, which docs/proto/v1/auth.proto declares already")
	require.NoError(t, rules.IdentifiersFree(r, n, planDomain(r, templates.FS(), n, []string{"rest", "graphql"}).plan))
}

// sharedRepo holds the shared files and nothing of task's own, so only what those files declare
// for task, its sentinels in errors.go and, with GraphQL, its resolver aliases, tell new-domain
// that task is taken.
func TestNewDomainChecksTheIdentifiersBeforeWritingAnything(t *testing.T) {
	t.Parallel()

	for name, tc := range map[string]struct {
		transports []string
		clash      string
	}{
		"every transport": {rules.Transports(), "internal/controller/graphql/resolver.go would declare taskQuery twice"},
		"no GraphQL":      {[]string{"rest", "grpc"}, "internal/entity/errors.go would declare ErrTaskNotFound twice"},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			dir := sharedRepo(t)
			before := fingerprint(t, dir)

			err := newDomain(t.Context(), &domainRequest{root: dir, name: "task", transports: tc.transports}, &bytes.Buffer{})

			require.ErrorIs(t, err, names.ErrInvalid)
			assert.Contains(t, err.Error(), tc.clash)
			assert.Equal(t, before, fingerprint(t, dir))
		})
	}
}

// The files of the GraphQL schema share its Query and Mutation, so a name free of files can still
// clash there: all-task would add the field allTasks to Query, which task.graphqls has.
func TestNewDomainRefusesANameWhoseGraphQLFieldsTheSchemaHas(t *testing.T) {
	t.Parallel()

	r := thisRepo()
	n := parseName(t, "all-task")
	require.NoError(t, rules.DomainFree(r, n), "no file, folder, service or type is called all-task")

	err := rules.IdentifiersFree(r, n, planDomain(r, templates.FS(), n, []string{"graphql"}).plan)

	require.ErrorIs(t, err, names.ErrInvalid)
	assert.Contains(t, err.Error(), "internal/controller/graphql/schema/all_task.graphqls would declare Query.allTasks, "+
		"which internal/controller/graphql/schema/task.graphqls declares already")
	require.NoError(t, rules.IdentifiersFree(r, n, planDomain(r, templates.FS(), n, []string{"rest"}).plan))
}

func TestEveryCreatedFileMatchesItsGolden(t *testing.T) {
	t.Parallel()

	pl := planWidget(t)
	created := createdFiles(t, pl.plan)
	require.NotEmpty(t, created)

	golden(t, filepath.Join(_goldenDir, _nextGolden+".golden"), []byte(pl.next))

	for p, content := range created {
		t.Run(p, func(t *testing.T) {
			t.Parallel()

			golden(t, filepath.Join(_goldenDir, filepath.FromSlash(p)+".golden"), content)
		})
	}

	created[_nextGolden] = []byte(pl.next)
	staleGoldens(t, _goldenDir, created)
}

// On only some transports, the files that differ from their every-transport golden match goldens
// of their own: the integration test among them, since it tests only the transports asked for.
func TestEveryCreatedFileMatchesItsGoldenOnSomeTransports(t *testing.T) {
	t.Parallel()

	every := createdFiles(t, planWidget(t).plan)

	for _, transports := range someTransports() {
		dir := _goldenDir + "-" + strings.Join(transports, "-")

		t.Run(dir, func(t *testing.T) {
			t.Parallel()

			differs := map[string][]byte{}

			for p, content := range createdFiles(t, planDomain(thisRepo(), templates.FS(), parseName(t, "widget"), transports).plan) {
				if !bytes.Equal(content, every[p]) {
					differs[p] = content
				}
			}

			require.Contains(t, differs, "integration-test/widget_test.go")

			for p, content := range differs {
				golden(t, filepath.Join(dir, filepath.FromSlash(p)+".golden"), content)
			}

			staleGoldens(t, dir, differs)
		})
	}
}

// staleGoldens fails for each golden in dir that matches none of the created files, left over from
// a template that moved or went away; UPDATE_GOLDEN=1 removes them instead.
func staleGoldens(t *testing.T, dir string, created map[string][]byte) {
	t.Helper()

	var stale []string

	require.NoError(t, filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if p == dir && errors.Is(err, fs.ErrNotExist) {
			return nil // the subtests report every golden missing
		}

		if err != nil || d.IsDir() {
			return err
		}

		rel, err := filepath.Rel(dir, p)
		if _, ok := created[strings.TrimSuffix(filepath.ToSlash(rel), ".golden")]; !ok {
			stale = append(stale, p)
		}

		return err
	}))

	for _, p := range stale {
		if os.Getenv("UPDATE_GOLDEN") != "" {
			require.NoError(t, os.Remove(p))

			continue
		}

		t.Errorf("%s matches no created file: UPDATE_GOLDEN=1 removes it", p)
	}
}

// The templates are written as their files for widget, on every transport and on only some, so
// formatting the Go files they render changes nothing for it. For other names it realigns
// columns, which the plan does.
func TestEveryTemplateReadsAsItsFileForWidget(t *testing.T) {
	t.Parallel()

	for _, transports := range append([][]string{rules.Transports()}, someTransports()...) {
		pl := planDomain(thisRepo(), templates.FS(), parseName(t, "widget"), transports)
		created := createdFiles(t, pl.plan)

		for tmpl := range pl.used {
			if strings.HasPrefix(tmpl, path.Join(_domainSet, _editsDir)+"/") || tmpl == path.Join(_domainSet, _nextTemplate) {
				continue
			}

			f, err := templates.Render(templates.FS(), tmpl, pl.data)
			require.NoError(t, err)
			assert.Equal(t, string(created[f.Path]), string(f.Content),
				"%s does not render go/format-clean for widget on %v: write it as the file gofmt would", tmpl, transports)
		}
	}
}

func TestEveryTemplateIsUsedByALayer(t *testing.T) {
	t.Parallel()

	pl := planWidget(t)

	require.NoError(t, fs.WalkDir(templates.FS(), _domainSet, func(p string, d fs.DirEntry, err error) error {
		if err == nil && !d.IsDir() {
			assert.True(t, pl.used[p], "no layer of new-domain uses %s", p)
		}

		return err
	}))
}

func TestCreatedGoFilesAlignForALongerName(t *testing.T) {
	t.Parallel()

	pl := planDomain(thisRepo(), templates.FS(), parseName(t, "line-item"), rules.Transports())

	require.Empty(t, pl.errs)
	require.NoError(t, rules.Valid(pl.plan))
	assert.Contains(t, string(createdFiles(t, pl.plan)["internal/entity/line_item_events.go"]),
		"\tLineItemID string    `json:\"line_item_id\"`\n\tUserID     string    `json:\"user_id\"`\n")
}

// Each transport's layer creates the files under its controller folder, and any it has elsewhere,
// and edits its shared files, such as its router; a domain without the transport gets none of them.
// The integration test, which every domain gets, tests the transport only when the domain has it.
func TestATransportLayerRunsOnlyForItsTransport(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		transport, dir string
		files          int
		elsewhere      []string
		edited         []string
		test           string
	}{
		{
			transport: "rest", dir: "internal/controller/restapi/v1/widget/", files: 7,
			edited: []string{"internal/controller/restapi/v1/router.go"},
			test:   "func TestHTTPWidget",
		},
		{
			transport: "grpc", dir: "internal/controller/grpc/v1/widget/", files: 6,
			elsewhere: []string{"docs/proto/v1/widget.proto"},
			edited:    []string{"internal/controller/grpc/v1/router.go"},
			test:      "func TestGRPCWidget",
		},
		{
			transport: "graphql", dir: "internal/controller/graphql/widget/", files: 6,
			elsewhere: []string{"internal/controller/graphql/schema/widget.graphqls"},
			edited: []string{
				"internal/controller/graphql/resolver.go",
				"internal/controller/graphql/gqlerr/gqlerr.go",
				"internal/controller/graphql/gqlgen.yml",
			},
			test: "func TestGraphQLWidget",
		},
	} {
		t.Run(tc.transport, func(t *testing.T) {
			t.Parallel()

			const integration = "integration-test/widget_test.go"

			for _, transports := range [][]string{{tc.transport}, rules.Transports()} {
				p := planDomain(thisRepo(), templates.FS(), parseName(t, "widget"), transports).plan

				assert.Len(t, underDir(p.Created(), tc.dir), tc.files, "with %v", transports)
				assert.Subset(t, p.Created(), tc.elsewhere, "with %v", transports)
				assert.Subset(t, p.Edited(), tc.edited, "with %v", transports)
				assert.Contains(t, string(createdFiles(t, p)[integration]), tc.test, "with %v", transports)
			}

			var others []string

			for _, other := range rules.Transports() {
				if other != tc.transport {
					others = append(others, other)
				}
			}

			p := planDomain(thisRepo(), templates.FS(), parseName(t, "widget"), others).plan

			assert.Empty(t, underDir(p.Created(), tc.dir), "with %v", others)
			assert.Empty(t, among(tc.elsewhere, p.Created()), "with %v", others)
			assert.Empty(t, among(tc.edited, p.Edited()), "with %v", others)
			assert.Contains(t, p.Created(), integration, "with %v", others)
			assert.NotContains(t, string(createdFiles(t, p)[integration]), tc.test, "with %v", others)
		})
	}
}

// among returns those of paths that are in all.
func among(paths, all []string) []string {
	var in []string

	for _, p := range paths {
		if slices.Contains(all, p) {
			in = append(in, p)
		}
	}

	return in
}

// underDir returns the paths in dir, which ends in a slash.
func underDir(paths []string, dir string) []string {
	var in []string

	for _, p := range paths {
		if strings.HasPrefix(p, dir) {
			in = append(in, p)
		}
	}

	return in
}

func TestSelectLayersRunsTheSharedLayersAndTheTransportsAskedFor(t *testing.T) {
	t.Parallel()

	var ran []string

	mark := func(name string) func(*planner) { return func(*planner) { ran = append(ran, name) } }
	all := []layer{
		{plan: mark("entity")},
		{transport: "rest", plan: mark("rest")},
		{transport: "grpc", plan: mark("grpc")},
		{plan: mark("integration")},
	}

	for _, l := range selectLayers(all, []string{"grpc"}) {
		l.plan(nil)
	}

	assert.Equal(t, []string{"entity", "grpc", "integration"}, ran)
}

func TestModulePathReadsGoMod(t *testing.T) {
	t.Parallel()

	for name, tc := range map[string]struct {
		gomod string
		want  string
		err   error
	}{
		"plain":     {gomod: "module example.com/app\n\ngo 1.24\n", want: "example.com/app"},
		"quoted":    {gomod: "// the app\nmodule \"example.com/app\"\n", want: "example.com/app"},
		"no module": {gomod: "go 1.24\n", err: rules.ErrTargets},
		"no go.mod": {err: rules.ErrTargets},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			fsys := fstest.MapFS{}
			if tc.gomod != "" {
				fsys["go.mod"] = &fstest.MapFile{Data: []byte(tc.gomod)}
			}

			got, err := modulePath(rules.Repo{FS: fsys})

			require.ErrorIs(t, err, tc.err)
			assert.Equal(t, tc.want, got)
		})
	}
}

func TestPlanningKeepsAMissingFileAndAMissingAnchorForTheRules(t *testing.T) {
	t.Parallel()

	fsys := fstest.MapFS{
		"go.mod":                   {Data: []byte("module example.com/app\n")},
		"internal/entity/event.go": {Data: []byte("package entity\n\nfunc EventTypes() []Event { return nil }\n")},
	}

	pl := planDomain(rules.Repo{FS: fsys}, templates.FS(), parseName(t, "widget"), nil)

	require.Len(t, pl.errs, 4, "event.go lacks the anchor; errors.go, contracts.go and buses.go are missing")
	require.ErrorIs(t, errors.Join(pl.errs...), rules.ErrTargets, "errors.go is missing")
	assert.Contains(t, errors.Join(pl.errs...).Error(), "slices.Concat")
	assert.NotEmpty(t, pl.plan.Created(), "the files to create are still planned, for the dry run")
}

func TestNewDomainTwiceFailsR1TheSecondTimeAndWritesNothing(t *testing.T) {
	t.Parallel()

	dir := sharedRepo(t)

	var out bytes.Buffer
	require.NoError(t, newDomain(t.Context(), &domainRequest{root: dir, name: "invoice", transports: rules.Transports()}, &out))
	assert.Contains(t, out.String(), "created 43, edited 9")

	events, err := os.ReadFile(filepath.Join(dir, "internal", "entity", "event.go"))
	require.NoError(t, err)
	assert.Contains(t, string(events), "\t\tinvoiceEvents(),\n\t)")

	router, err := os.ReadFile(filepath.Join(dir, "internal", "controller", "restapi", "v1", "router.go"))
	require.NoError(t, err)
	assert.Contains(t, string(router), "\tinvoice.NewRoutes(api, b, l)\n}", "after the last NewRoutes call")
	assert.Contains(t, string(router), "\t\"github.com/m7medvision/go-clean-template/internal/controller/restapi/v1/invoice\"\n")

	grpcRouter, err := os.ReadFile(filepath.Join(dir, "internal", "controller", "grpc", "v1", "router.go"))
	require.NoError(t, err)
	assert.Contains(t, string(grpcRouter), "\tv1.RegisterInvoiceServiceServer(app, invoice.New(b, l))\n}",
		"after the last Register*ServiceServer call")
	assert.Contains(t, string(grpcRouter), "\t\"github.com/m7medvision/go-clean-template/internal/controller/grpc/v1/invoice\"\n")

	resolver, err := os.ReadFile(filepath.Join(dir, "internal", "controller", "graphql", "resolver.go"))
	require.NoError(t, err)

	for _, want := range []string{
		"\tinvoiceQuery    = invoice.Query\n\tinvoiceMutation = invoice.Mutation\n)",
		"\texchangeQuery\n\tinvoiceQuery\n}",
		"\ttaskMutation\n\tinvoiceMutation\n}",
		"\ttaskQ, taskM := task.New(b, l)\n\tinvoiceQ, invoiceM := invoice.New(b, l)\n",
		"exchangeQuery: exchange.New(b, l), invoiceQuery: invoiceQ}",
		"mutation{taskMutation: taskM, invoiceMutation: invoiceM}",
		"\t\"github.com/m7medvision/go-clean-template/internal/controller/graphql/invoice\"\n",
	} {
		assert.Contains(t, string(resolver), want)
	}

	gqlerr, err := os.ReadFile(filepath.Join(dir, "internal", "controller", "graphql", "gqlerr", "gqlerr.go"))
	require.NoError(t, err)
	assert.Contains(t, string(gqlerr), "\t\t{entity.ErrInvoiceNotFound, _codeNotFound, \"invoice not found\"},\n"+
		"\t\t{entity.ErrInvoiceForbidden, _codeForbidden, \"forbidden\"},\n\t\t// Any message can produce these three",
		"ahead of the shared entries")

	gqlgen, err := os.ReadFile(filepath.Join(dir, "internal", "controller", "graphql", "gqlgen.yml"))
	require.NoError(t, err)
	assert.Contains(t, string(gqlgen),
		"\n  Invoice:\n    model: github.com/m7medvision/go-clean-template/internal/entity.Invoice\n",
		"the last of the models")

	before := fingerprint(t, dir)

	err = newDomain(t.Context(), &domainRequest{root: dir, name: "invoice", transports: rules.Transports()}, &bytes.Buffer{})
	require.ErrorIs(t, err, names.ErrInvalid)
	assert.Equal(t, before, fingerprint(t, dir))
}

// sharedRepo is a committed repository holding this repository's go.mod and every shared file
// new-domain edits, copied as they are on disk, so a write against it meets the real anchors.
func sharedRepo(t *testing.T) string {
	t.Helper()

	files := map[string]string{}

	for _, p := range append(planWidget(t).plan.Edited(), "go.mod") {
		b, err := os.ReadFile(filepath.Join("..", "..", filepath.FromSlash(p)))
		require.NoError(t, err)

		files[p] = string(b)
	}

	return gitRepoWith(t, files)
}

// golden compares got with the file at p; UPDATE_GOLDEN=1 rewrites the file instead.
func golden(t *testing.T, p string, got []byte) {
	t.Helper()

	if os.Getenv("UPDATE_GOLDEN") != "" {
		require.NoError(t, os.MkdirAll(filepath.Dir(p), 0o750))
		require.NoError(t, os.WriteFile(p, got, 0o600))
	}

	want, err := os.ReadFile(p)
	require.NoError(t, err, "run UPDATE_GOLDEN=1 go -C tools/gen test ./... and review the diff")
	assert.Equal(t, string(want), string(got))
}
