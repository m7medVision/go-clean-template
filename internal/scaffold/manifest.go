// Package scaffold generates a project from this template with only the features picked: it copies
// the repository, drops the paths of the features left out, strips their marked blocks from shared
// files, renames the module and regenerates the generated code. scaffold/features.yaml drives it.
package scaffold

import (
	"errors"
	"fmt"
	"os"
	"slices"

	"go.yaml.in/yaml/v3"
)

// ManifestPath is where the manifest lives, relative to the template's root.
const ManifestPath = "scaffold/features.yaml"

// SelfFeature names the scaffolder itself in markers and paths. It is never kept, so a generated
// project does not ship the scaffolder.
const SelfFeature = "self"

var (
	// ErrInvalidManifest is a features.yaml that breaks its own rules.
	ErrInvalidManifest = errors.New("invalid manifest")
	// ErrInvalidSelection is a choice of features that the manifest rejects.
	ErrInvalidSelection = errors.New("invalid selection")
)

// Manifest is scaffold/features.yaml.
type Manifest struct {
	// Module is the template's Go module path, replaced by the generated project's.
	Module string `yaml:"module"`
	// Self lists the scaffolder's own paths; they never reach a generated project.
	Self []string `yaml:"self"`
	// Core lists the paths that belong to every project. A file matching neither Core, Self nor a
	// feature's Paths is unclaimed, which the repository test rejects.
	Core []string `yaml:"core"`
	// Generated lists files rebuilt by Regen; the leak check skips them.
	Generated []string  `yaml:"generated"`
	Features  []Feature `yaml:"features"`
	Presets   []Preset  `yaml:"presets"`
	// Regen runs in order inside the generated project.
	Regen []Step `yaml:"regen"`
}

// Feature is one thing a generated project can leave out.
type Feature struct {
	ID          string   `yaml:"id"`
	Label       string   `yaml:"label"`
	Description string   `yaml:"description"`
	Requires    []string `yaml:"requires"`
	// Core features are always kept.
	Core bool `yaml:"core"`
	// Paths are owned by the feature: a project without it has none of them. A path listed under
	// several features is kept only when all of them are.
	Paths []string `yaml:"paths"`
	// Leaks are words that must not appear in a project without the feature, outside Generated.
	Leaks []string `yaml:"leaks"`
	// Allow exempts lines from Leaks: a line matching one of these may mention the feature.
	Allow []string `yaml:"allow"`
}

// Preset is a named set of features the TUI and `make scaffold PRESET=` offer.
type Preset struct {
	ID       string   `yaml:"id"`
	Label    string   `yaml:"label"`
	Features []string `yaml:"features"`
}

// Step is one regeneration command, run only when its feature is kept (always, when When is empty).
type Step struct {
	Run  string `yaml:"run"`
	When string `yaml:"when"`
}

// Load reads and validates the manifest at path.
func Load(path string) (*Manifest, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("scaffold - Load - os.ReadFile: %w", err)
	}

	return Parse(data)
}

// Parse decodes and validates a manifest.
func Parse(data []byte) (*Manifest, error) {
	var m Manifest
	if err := yaml.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("%w: yaml: %w", ErrInvalidManifest, err)
	}

	if err := m.validate(); err != nil {
		return nil, err
	}

	return &m, nil
}

// Feature returns the feature with id.
func (m *Manifest) Feature(id string) (Feature, bool) {
	i := slices.IndexFunc(m.Features, func(f Feature) bool { return f.ID == id })
	if i < 0 {
		return Feature{}, false
	}

	return m.Features[i], true
}

// Known reports whether id names a feature or the scaffolder itself.
func (m *Manifest) Known(id string) bool {
	_, ok := m.Feature(id)

	return ok || id == SelfFeature
}

func (m *Manifest) validate() error {
	seen := map[string]bool{}

	for i := range m.Features {
		f := &m.Features[i]

		if f.ID == SelfFeature {
			return fmt.Errorf("%w: feature id %q is reserved", ErrInvalidManifest, SelfFeature)
		}

		if seen[f.ID] {
			return fmt.Errorf("%w: feature %q is declared twice", ErrInvalidManifest, f.ID)
		}

		seen[f.ID] = true
	}

	for i := range m.Features {
		f := &m.Features[i]

		for _, r := range f.Requires {
			if !seen[r] {
				return fmt.Errorf("%w: feature %q requires unknown feature %q", ErrInvalidManifest, f.ID, r)
			}
		}
	}

	if err := m.checkCycles(); err != nil {
		return err
	}

	if err := m.validateRegen(seen); err != nil {
		return err
	}

	return m.validatePresets(seen)
}

func (m *Manifest) validateRegen(features map[string]bool) error {
	for _, step := range m.Regen {
		if step.When != "" && !features[step.When] {
			return fmt.Errorf("%w: regen step %q names unknown feature %q", ErrInvalidManifest, step.Run, step.When)
		}
	}

	return nil
}

func (m *Manifest) validatePresets(features map[string]bool) error {
	for _, p := range m.Presets {
		for _, id := range p.Features {
			if !features[id] {
				return fmt.Errorf("%w: preset %q lists unknown feature %q", ErrInvalidManifest, p.ID, id)
			}
		}

		if problems := m.Problems(append(m.coreIDs(), p.Features...)); len(problems) > 0 {
			return fmt.Errorf("%w: preset %q: %s", ErrInvalidManifest, p.ID, problems[0])
		}
	}

	return nil
}

// checkCycles walks the requirement graph depth first; a feature met again on its own path is a cycle.
func (m *Manifest) checkCycles() error {
	const (
		unvisited = iota
		visiting
		done
	)

	state := map[string]int{}

	var visit func(id string) error

	visit = func(id string) error {
		switch state[id] {
		case visiting:
			return fmt.Errorf("%w: requirement cycle through %q", ErrInvalidManifest, id)
		case done:
			return nil
		}

		state[id] = visiting

		f, _ := m.Feature(id)
		for _, r := range f.Requires {
			if err := visit(r); err != nil {
				return err
			}
		}

		state[id] = done

		return nil
	}

	for i := range m.Features {
		f := &m.Features[i]

		if err := visit(f.ID); err != nil {
			return err
		}
	}

	return nil
}

func (m *Manifest) coreIDs() []string {
	var ids []string

	for i := range m.Features {
		f := &m.Features[i]

		if f.Core {
			ids = append(ids, f.ID)
		}
	}

	return ids
}

// Claimed reports whether name belongs to core, the scaffolder, or a feature.
func (m *Manifest) Claimed(name string) bool {
	if matchAny(m.Core, name) || matchAny(m.Self, name) {
		return true
	}

	return slices.ContainsFunc(m.Features, func(f Feature) bool { return matchAny(f.Paths, name) })
}

// IsGenerated reports whether name is rebuilt by the regen steps.
func (m *Manifest) IsGenerated(name string) bool {
	return matchAny(m.Generated, name)
}

// Dependents lists, in manifest order, every feature that needs id directly or through another
// feature: dropping id drops them too.
func (m *Manifest) Dependents(id string) []string {
	needs := map[string]bool{id: true}

	// Requirements point at earlier or later features, so repeat until nothing new is found.
	for changed := true; changed; {
		changed = false

		for i := range m.Features {
			f := &m.Features[i]
			if needs[f.ID] || !slices.ContainsFunc(f.Requires, func(r string) bool { return needs[r] }) {
				continue
			}

			needs[f.ID], changed = true, true
		}
	}

	var ids []string

	for i := range m.Features {
		if f := &m.Features[i]; f.ID != id && needs[f.ID] {
			ids = append(ids, f.ID)
		}
	}

	return ids
}
