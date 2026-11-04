package main

import (
	"errors"
	"fmt"
	"go/format"
	"io/fs"
	"path"
	"slices"
	"strconv"
	"strings"

	"github.com/m7medvision/go-clean-template/tools/gen/edit"
	"github.com/m7medvision/go-clean-template/tools/gen/plan"
	"github.com/m7medvision/go-clean-template/tools/gen/rules"
	"github.com/m7medvision/go-clean-template/tools/gen/templates"
)

const (
	// _domainSet is the set of templates new-domain renders, templates/domain.
	_domainSet = "domain"
	// _editsDir holds, inside a set, the text of the insertions into shared files.
	_editsDir = "edits"
)

// layer is one part of what a command generates, such as the entity layer of a domain. Each lives
// in its own layer_<name>.go as a function that adds to the planner the files it creates and the
// edits it makes; a command lists its layers (domainLayers) and runs them in order.
type layer struct {
	// transport is the transport the layer belongs to, such as rest; empty for a layer every
	// domain gets.
	transport string
	plan      func(pl *planner)
}

// selectLayers returns the layers of all that run for transports: the shared ones and those of the
// transports asked for, in their order in all.
func selectLayers(all []layer, transports []string) []layer {
	var run []layer

	for _, l := range all {
		if l.transport == "" || slices.Contains(transports, l.transport) {
			run = append(run, l)
		}
	}

	return run
}

// planner is what each layer plans with: the repository it reads, the templates and the data it
// renders them with, and the plan it adds to.
type planner struct {
	repo rules.Repo
	tmpl fs.FS
	data *templates.Data
	plan *plan.Plan
	// errs are what planning ran into: a file to edit that does not exist (R2), a place not found
	// exactly once (R4), a template that does not render. They are kept rather than returned, so
	// the dry run still shows the plan and rules.Check reports the first rule in order.
	errs []error
	// used holds every template rendered, so a test can find any that no layer uses.
	used map[string]bool
}

func newPlanner(r rules.Repo, tmpl fs.FS, title string, data *templates.Data) *planner {
	return &planner{repo: r, tmpl: tmpl, data: data, plan: plan.New(title), used: map[string]bool{}}
}

func (pl *planner) fail(err error) {
	pl.errs = append(pl.errs, err)
}

// create plans the files that the templates matching pattern, a fs.Glob pattern under
// templates/domain such as "internal/entity/*.tmpl", render to.
//
// A created Go file goes through go/format: gofmt aligns columns by the longest name in them, so
// how a file aligns depends on the domain's name. Templates are written as their files for widget,
// which a test checks. Output that does not parse is planned as it is, for rule R5 to report.
func (pl *planner) create(pattern string) {
	files, err := templates.Glob(pl.tmpl, path.Join(_domainSet, pattern), pl.data)
	if err != nil {
		pl.fail(err)

		return
	}

	for _, f := range files {
		pl.used[f.Template] = true

		content := f.Content
		if path.Ext(f.Path) == ".go" {
			if formatted, err := format.Source(content); err == nil {
				content = formatted
			}
		}

		pl.plan.Create(f.Path, content)
	}
}

// text renders templates/domain/edits/<name>.tmpl, the text of one insertion. name is the shared
// file's repository path and the place in it, as in "internal/entity/event.go.EventTypes".
func (pl *planner) text(name string) string {
	tmpl := path.Join(_domainSet, _editsDir, name+".tmpl")
	pl.used[tmpl] = true

	text, err := templates.Text(pl.tmpl, tmpl, pl.data)
	if err != nil {
		pl.fail(err)
	}

	return text
}

// editFile plans the insertions that edits return into the shared file at p, parsed with parse
// (edit.ParseGo, edit.ParseProto, edit.ParseGraphQL or edit.ParseYAML). A file that does not exist
// or parse, and an edit that fails, go to pl.errs; the other edits are still planned.
func editFile[F any](pl *planner, p string, parse func(string, []byte) (F, error), edits ...func(F) ([]edit.Insertion, error)) {
	src, err := pl.repo.Read(p)
	if err != nil {
		pl.fail(err)

		return
	}

	parsed, err := parse(p, src)
	if err != nil {
		pl.fail(err)

		return
	}

	for _, e := range edits {
		ins, err := e(parsed)
		if err != nil {
			pl.fail(err)

			continue
		}

		if len(ins) > 0 {
			pl.plan.Edit(p, src, ins...)
		}
	}
}

// one adapts an editor that returns one insertion to editFile.
func one(in edit.Insertion, err error) ([]edit.Insertion, error) {
	if err != nil {
		return nil, err
	}

	return []edit.Insertion{in}, nil
}

// modulePath is the module path the repository's go.mod declares, which every import of the app
// starts with.
func modulePath(r rules.Repo) (string, error) {
	src, err := fs.ReadFile(r.FS, "go.mod")
	if errors.Is(err, fs.ErrNotExist) {
		return "", fmt.Errorf("%w: there is no go.mod: is --root the repository root?", rules.ErrTargets)
	}

	if err != nil {
		return "", fmt.Errorf("read go.mod: %w", err)
	}

	for line := range strings.Lines(string(src)) {
		fields := strings.Fields(line)
		if len(fields) != 2 || fields[0] != "module" {
			continue
		}

		if unquoted, err := strconv.Unquote(fields[1]); err == nil {
			return unquoted, nil
		}

		return fields[1], nil
	}

	return "", fmt.Errorf("%w: go.mod declares no module", rules.ErrTargets)
}
