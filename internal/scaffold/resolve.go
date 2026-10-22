package scaffold

import (
	"fmt"
	"slices"
	"strings"
)

// Selection is what the user picked: a preset, then features added and removed. With no preset,
// the start is every feature, unless With is given alone, which starts from the core features
// (as does CustomPreset).
type Selection struct {
	Preset  string
	With    []string
	Without []string
}

// Set is the resolved features of a generated project.
type Set struct {
	ids []string
}

// Has reports whether the project keeps the feature. The scaffolder itself is never kept.
func (s Set) Has(id string) bool {
	return slices.Contains(s.ids, id)
}

// IDs lists the kept features in manifest order.
func (s Set) IDs() []string {
	return slices.Clone(s.ids)
}

// Resolve turns a selection into the set of features to keep, or explains why it is invalid.
func (m *Manifest) Resolve(sel Selection) (Set, error) {
	start, err := m.start(sel)
	if err != nil {
		return Set{}, err
	}

	chosen := map[string]bool{}
	for _, id := range append(start, m.coreIDs()...) {
		chosen[id] = true
	}

	if err := m.apply(chosen, sel); err != nil {
		return Set{}, err
	}

	var set Set

	for i := range m.Features {
		f := &m.Features[i]

		if chosen[f.ID] {
			set.ids = append(set.ids, f.ID)
		}
	}

	if problems := m.Problems(set.ids); len(problems) > 0 {
		return Set{}, fmt.Errorf("%w: %s", ErrInvalidSelection, strings.Join(problems, "; "))
	}

	return set, nil
}

// apply adds sel.With to chosen and removes sel.Without.
func (m *Manifest) apply(chosen map[string]bool, sel Selection) error {
	for _, id := range sel.With {
		if _, ok := m.Feature(id); !ok {
			return fmt.Errorf("%w: unknown feature %q", ErrInvalidSelection, id)
		}

		chosen[id] = true
	}

	for _, id := range sel.Without {
		f, ok := m.Feature(id)
		if !ok {
			return fmt.Errorf("%w: unknown feature %q", ErrInvalidSelection, id)
		}

		if f.Core {
			return fmt.Errorf("%w: %q is always included", ErrInvalidSelection, id)
		}

		delete(chosen, id)
	}

	return nil
}

func (m *Manifest) start(sel Selection) ([]string, error) {
	if sel.Preset != "" {
		return m.PresetFeatures(sel.Preset)
	}

	if len(sel.With) > 0 && len(sel.Without) == 0 {
		return nil, nil
	}

	ids := make([]string, 0, len(m.Features))
	for i := range m.Features {
		f := &m.Features[i]

		ids = append(ids, f.ID)
	}

	return ids, nil
}

// Problems lists, in manifest order, every chosen feature whose requirement is not chosen too:
// "web needs graphql".
func (m *Manifest) Problems(chosen []string) []string {
	var problems []string

	for i := range m.Features {
		f := &m.Features[i]

		if !slices.Contains(chosen, f.ID) {
			continue
		}

		for _, r := range f.Requires {
			if !slices.Contains(chosen, r) {
				problems = append(problems, f.ID+" needs "+r)
			}
		}
	}

	return problems
}
