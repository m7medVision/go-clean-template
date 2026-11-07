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
	"text/template"

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
	r.Status = func(context.Context) (rules.Status, error) { return rules.Status{}, nil }

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

	return planOf(t, thisRepo(), parseName(t, "widget"), rules.Transports())
}

// planOf plans the domain n on transports against the repository r, which has a go.mod.
func planOf(t *testing.T, r rules.Repo, n *names.Name, transports []string) *planner {
	t.Helper()

	pl, err := planDomain(r, n, transports)
	require.NoError(t, err)

	return pl
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
	require.NoError(t, rules.DomainFree(t.Context(), r, parseName(t, "widget")))

	pl := planWidget(t)

	require.Empty(t, pl.errs)
	require.NoError(t, rules.IdentifiersFree(r, parseName(t, "widget"), pl.plan))
	require.NoError(t, rules.Check(t.Context(), r, pl.plan, pl.errs))
}

// A name can own no file and still clash with what a package declares: record would declare the
// type entity.Record, and the entity package has a function called Record.
func TestNewDomainRefusesANameWhoseIdentifiersAPackageHas(t *testing.T) {
	t.Parallel()

	r := thisRepo()
	n := parseName(t, "record")
	require.NoError(t, rules.DomainFree(t.Context(), r, n), "no file or folder is called record")

	err := rules.IdentifiersFree(r, n, planOf(t, r, n, rules.Transports()).plan)

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
	require.NoError(t, rules.DomainFree(t.Context(), r, n), "no file, folder or service is called login")

	err := rules.IdentifiersFree(r, n, planOf(t, r, n, rules.Transports()).plan)

	require.ErrorIs(t, err, names.ErrInvalid)
	assert.Contains(t, err.Error(), "docs/proto/v1/login.proto would declare LoginResponse, which docs/proto/v1/auth.proto declares already")
	require.NoError(t, rules.IdentifiersFree(r, n, planOf(t, r, n, []string{"rest", "graphql"}).plan))
}

// sharedRepo holds the shared files and nothing of task's own, so only what those files declare
// for task, its resolver aliases, tell new-domain that task is taken.
func TestNewDomainChecksTheIdentifiersBeforeWritingAnything(t *testing.T) {
	t.Parallel()

	dir := sharedRepo(t)
	before := fingerprint(t, dir)

	err := newDomain(t.Context(), &domainRequest{root: dir, name: "task", transports: rules.Transports()}, &bytes.Buffer{})

	require.ErrorIs(t, err, names.ErrInvalid)
	assert.Contains(t, err.Error(), "internal/controller/graphql/resolver.go would declare taskQuery twice")
	assert.Equal(t, before, fingerprint(t, dir))
}

// Each GraphQL field has its own schema file, named after it, so a name free of every folder and
// type can still give a field the schema has: all-task would add the field allTasks to Query in
// all_tasks.graphqls, which is task's.
func TestNewDomainRefusesANameWhoseGraphQLFieldsTheSchemaHas(t *testing.T) {
	t.Parallel()

	r := thisRepo()
	n := parseName(t, "all-task")
	require.NoError(t, rules.DomainFree(t.Context(), r, n), "no file, folder, service or type is called all-task")

	pl := planOf(t, r, n, []string{"graphql"})
	err := rules.Check(t.Context(), r, pl.plan, pl.errs)

	require.ErrorIs(t, err, rules.ErrTargets)
	assert.Contains(t, err.Error(), "internal/controller/graphql/schema/all_tasks.graphqls, a file to create, exists")

	pl = planOf(t, r, n, []string{"rest"})
	require.NoError(t, rules.Check(t.Context(), r, pl.plan, pl.errs))
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

			for p, content := range createdFiles(t, planOf(t, thisRepo(), parseName(t, "widget"), transports).plan) {
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
		readAsTheirFiles(t, planOf(t, thisRepo(), parseName(t, "widget"), transports))
	}
}

// readAsTheirFiles checks that every template of pl's set that renders a file pl creates renders
// it as pl has it, after go/format: formatting changed nothing.
func readAsTheirFiles(t *testing.T, pl *planner) {
	t.Helper()

	created := createdFiles(t, pl.plan)

	for _, tmpl := range setTemplates(t, pl.set) {
		if kind(tmpl) != _createsAFile {
			continue
		}

		f := renderFile(t, pl.tmpl, tmpl, pl.data)
		if content, ok := created[f.Path]; ok {
			assert.Equal(t, string(content), string(f.Content),
				"%s does not render go/format-clean for %s: write it as the file gofmt would", tmpl, pl.plan.Title())
		}
	}
}

func TestEveryTemplateIsUsedByALayer(t *testing.T) {
	t.Parallel()

	used := usedTemplates(t, planWidget(t))

	for _, tmpl := range setTemplates(t, _domainSet) {
		assert.True(t, used[path.Join(_domainSet, tmpl)], "no layer of new-domain uses %s", tmpl)
	}
}

// The kinds of template in a set.
const (
	_createsAFile = iota
	_insertsText
	_isTheChecklist
)

// kind is the kind of the template tmpl, a path in its set: one that creates a file, one that holds
// the text of an insertion, or the checklist.
func kind(tmpl string) int {
	switch {
	case tmpl == _nextTemplate:
		return _isTheChecklist
	case strings.HasPrefix(tmpl, _editsDir+"/"):
		return _insertsText
	default:
		return _createsAFile
	}
}

// setTemplates returns the path in set of every template of set.
func setTemplates(t *testing.T, set string) []string {
	t.Helper()

	fsys, err := templates.Set(set)
	require.NoError(t, err)

	var all []string

	require.NoError(t, fs.WalkDir(fsys, ".", func(p string, d fs.DirEntry, err error) error {
		if err == nil && !d.IsDir() {
			all = append(all, p)
		}

		return err
	}))

	return all
}

// renderFile renders the template tmpl of the set fsys, which creates a file, with data.
func renderFile(t *testing.T, fsys fs.FS, tmpl string, data *templates.Data) templates.File {
	t.Helper()

	files, err := templates.Glob(fsys, tmpl, data)
	require.NoError(t, err)
	require.Len(t, files, 1, tmpl)

	return files[0]
}

// usedTemplates returns the templates of the set plans render whose output one of plans holds,
// from what each plan creates and edits: a template that creates a file renders the path of one
// the plan creates; an insertion's renders text that the file it names holds, as the plan leaves
// it, whitespace aside, since go/format realigns it, or the plan ran into an error at its place in
// that file, which a layer looked for; and the checklist is the plan's.
func usedTemplates(t *testing.T, plans ...*planner) map[string]bool {
	t.Helper()

	used := map[string]bool{}

	for _, pl := range plans {
		changes, err := pl.plan.Changes()
		require.NoError(t, err)

		after := map[string]string{}
		for _, c := range changes {
			after[c.Path] = squeezed(string(c.New))
		}

		for _, tmpl := range setTemplates(t, pl.set) {
			key := path.Join(pl.set, tmpl)
			used[key] = used[key] || usedBy(t, pl, tmpl, after)
		}
	}

	return used
}

// usedBy reports whether the plan pl, which leaves the files at their paths as after holds them,
// whitespace squeezed out, holds the output of the template tmpl.
func usedBy(t *testing.T, pl *planner, tmpl string, after map[string]string) bool {
	t.Helper()

	switch kind(tmpl) {
	case _isTheChecklist:
		text, err := templates.Text(pl.tmpl, tmpl, pl.data)
		require.NoError(t, err)

		return pl.next == text+"\n"
	case _insertsText:
		text, err := templates.Text(pl.tmpl, tmpl, pl.data)
		require.NoError(t, err)

		edited, place := editedPlace(t, pl, tmpl)
		file, ok := after[edited]

		return ok && squeezed(text) != "" && strings.Contains(file, squeezed(text)) ||
			slices.ContainsFunc(pl.errs, func(err error) bool {
				return strings.Contains(err.Error(), edited+":") && strings.Contains(err.Error(), place)
			})
	default:
		_, ok := after[renderFile(t, pl.tmpl, tmpl, pl.data).Path]

		return ok
	}
}

// editedPlace is the repository path of the file the insertion template tmpl names, its name past
// the set's edits folder up to the edited file's extension, rendered with pl's data; and the first
// word of the place in that file it names next, as in subscribers for
// internal/app/buses.go.subscribers.
func editedPlace(t *testing.T, pl *planner, tmpl string) (edited, place string) {
	t.Helper()

	name := strings.TrimSuffix(strings.TrimPrefix(tmpl, _editsDir+"/"), ".tmpl")

	end := -1

	for _, ext := range []string{".go.", ".proto.", ".graphqls.", ".yml."} {
		if i := strings.Index(name, ext); i >= 0 && (end < 0 || i+len(ext)-1 < end) {
			end = i + len(ext) - 1
		}
	}

	require.Positive(t, end, "%s names no edited file", tmpl)

	parsed, err := template.New(tmpl).Option("missingkey=error").Parse(name[:end])
	require.NoError(t, err)

	var p strings.Builder
	require.NoError(t, parsed.Execute(&p, pl.data))

	place, _, _ = strings.Cut(name[end+1:], ".")

	return p.String(), place
}

// squeezed is s without its whitespace.
func squeezed(s string) string {
	return strings.Join(strings.Fields(s), "")
}

func TestCreatedGoFilesAlignForALongerName(t *testing.T) {
	t.Parallel()

	pl := planOf(t, thisRepo(), parseName(t, "line-item"), rules.Transports())

	require.Empty(t, pl.errs)
	require.NoError(t, rules.Check(t.Context(), thisRepo(), pl.plan, pl.errs))
	assert.Contains(t, string(createdFiles(t, pl.plan)["internal/entity/line_item_events.go"]),
		"\tLineItemID string    `json:\"line_item_id\"`\n\tUserID     string    `json:\"user_id\"`\n")
}

// Each transport's layer creates the files under its controller folder, and any it has elsewhere,
// and edits its shared files, as GraphQL's root resolver; a domain without the transport gets none
// of them.
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
			transport: "rest", dir: "internal/controller/restapi/v1/widget/", files: 10,
			test: "func TestHTTPWidget",
		},
		{
			transport: "grpc", dir: "internal/controller/grpc/v1/widget/", files: 8,
			elsewhere: []string{"docs/proto/v1/widget.proto"},
			test:      "func TestGRPCWidget",
		},
		{
			transport: "graphql", dir: "internal/controller/graphql/widget/", files: 9,
			elsewhere: []string{
				"internal/controller/graphql/schema/widget_types.graphqls",
				"internal/controller/graphql/schema/widget.graphqls",
				"internal/controller/graphql/schema/widgets.graphqls",
				"internal/controller/graphql/schema/create_widget.graphqls",
				"internal/controller/graphql/schema/update_widget.graphqls",
				"internal/controller/graphql/schema/delete_widget.graphqls",
			},
			edited: []string{"internal/controller/graphql/resolver.go"},
			test:   "func TestGraphQLWidget",
		},
	} {
		t.Run(tc.transport, func(t *testing.T) {
			t.Parallel()

			const integration = "integration-test/widget_test.go"

			for _, transports := range [][]string{{tc.transport}, rules.Transports()} {
				p := planOf(t, thisRepo(), parseName(t, "widget"), transports).plan

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

			p := planOf(t, thisRepo(), parseName(t, "widget"), others).plan

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
		{edits: mark("entity")},
		{transport: "rest", edits: mark("rest")},
		{transport: "grpc", edits: mark("grpc")},
		{edits: mark("integration")},
	}

	for _, l := range selectLayers(all, []string{"grpc"}) {
		l.edits(nil)
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

// Without the module go.mod declares, no import of the app can be written, so planning stops there
// rather than plan imports without it.
func TestPlanningStopsWithoutTheModule(t *testing.T) {
	t.Parallel()

	_, err := planDomain(rules.Repo{FS: fstest.MapFS{}}, parseName(t, "widget"), nil)

	require.ErrorIs(t, err, rules.ErrTargets)
	assert.Contains(t, err.Error(), "there is no go.mod")
}

func TestPlanningKeepsAMissingFileAndAMissingAnchorForTheRules(t *testing.T) {
	t.Parallel()

	fsys := fstest.MapFS{
		"go.mod":                   {Data: []byte("module example.com/app\n")},
		"internal/entity/event.go": {Data: []byte("package entity\n\nfunc EventTypes() []Event { return nil }\n")},
	}

	pl := planOf(t, rules.Repo{FS: fsys}, parseName(t, "widget"), nil)

	require.Len(t, pl.errs, 2, "event.go lacks the anchor; modules.go is missing")
	require.ErrorIs(t, errors.Join(pl.errs...), rules.ErrTargets, "modules.go is missing")
	assert.Contains(t, errors.Join(pl.errs...).Error(), "slices.Concat")
	assert.NotEmpty(t, pl.plan.Created(), "the files to create are still planned, for the dry run")
}

func TestNewDomainTwiceFailsR1TheSecondTimeAndWritesNothing(t *testing.T) {
	t.Parallel()

	dir := sharedRepo(t)

	var out bytes.Buffer
	require.NoError(t, newDomain(t.Context(), &domainRequest{root: dir, name: "invoice", transports: rules.Transports()}, &out))
	assert.Contains(t, out.String(), "created 58, edited 3")

	events, err := os.ReadFile(filepath.Join(dir, "internal", "entity", "event.go"))
	require.NoError(t, err)
	assert.Contains(t, string(events), "\t\tInvoiceEvents(),\n\t)")

	modules, err := os.ReadFile(filepath.Join(dir, "internal", "app", "modules.go"))
	require.NoError(t, err)
	assert.Contains(t, string(modules), "\t\tinvoiceModule(),\n\t}\n", "the last of the modules")

	resolver, err := os.ReadFile(filepath.Join(dir, "internal", "controller", "graphql", "resolver.go"))
	require.NoError(t, err)

	for _, want := range []string{
		"\tinvoiceQuery    = invoice.Query\n\tinvoiceMutation = invoice.Mutation\n)",
		"\texchangeQuery\n\tinvoiceQuery\n}",
		"\ttaskMutation\n\tinvoiceMutation\n}",
		"exchange.New(b, l)\n\tr.query.invoiceQuery, r.mutation.invoiceMutation = invoice.New(b, l)\n",
		"\t\"github.com/m7medvision/go-clean-template/internal/controller/graphql/invoice\"\n",
	} {
		assert.Contains(t, string(resolver), want)
	}

	before := fingerprint(t, dir)

	err = newDomain(t.Context(), &domainRequest{root: dir, name: "invoice", transports: rules.Transports()}, &bytes.Buffer{})
	require.ErrorIs(t, err, names.ErrInvalid)
	assert.Equal(t, before, fingerprint(t, dir))
}

// sharedRepo is a committed repository holding this repository's go.mod, its OpenAPI document and
// every shared file new-domain edits, copied as they are on disk, so a write against it meets the
// real anchors, and a use case on it the real REST operations.
func sharedRepo(t *testing.T) string {
	t.Helper()

	files := map[string]string{}

	for _, p := range append(planWidget(t).plan.Edited(), "go.mod", "docs/openapi/v1.json") {
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
