package scaffold

import (
	"fmt"
	"slices"
	"strings"
)

// CustomPreset is the TUI's "pick every feature yourself" choice: it starts from the core features.
const CustomPreset = "custom"

// Choice is one optional feature as the TUI offers it.
type Choice struct {
	ID string
	// Label names the feature and what it needs: "Frontend (needs GraphQL)".
	Label string
}

// Choices lists the optional features in manifest order; core features are always included.
func (m *Manifest) Choices() []Choice {
	var choices []Choice

	for i := range m.Features {
		f := &m.Features[i]
		if f.Core {
			continue
		}

		label := f.label()
		if len(f.Requires) > 0 {
			needs := make([]string, 0, len(f.Requires))
			for _, r := range f.Requires {
				req, _ := m.Feature(r)
				needs = append(needs, req.label())
			}

			label += " (needs " + strings.Join(needs, ", ") + ")"
		}

		choices = append(choices, Choice{ID: f.ID, Label: label})
	}

	return choices
}

func (f *Feature) label() string {
	if f.Label != "" {
		return f.Label
	}

	return f.ID
}

// PresetFeatures returns the optional features a preset turns on; CustomPreset turns on none.
func (m *Manifest) PresetFeatures(id string) ([]string, error) {
	if id == CustomPreset {
		return nil, nil
	}

	i := slices.IndexFunc(m.Presets, func(p Preset) bool { return p.ID == id })
	if i < 0 {
		return nil, fmt.Errorf("%w: unknown preset %q", ErrInvalidSelection, id)
	}

	return slices.Clone(m.Presets[i].Features), nil
}

// ValidateChoices checks optional features picked in the TUI against their requirements.
func (m *Manifest) ValidateChoices(chosen []string) error {
	if problems := m.Problems(append(m.coreIDs(), chosen...)); len(problems) > 0 {
		return fmt.Errorf("%w: %s", ErrInvalidSelection, strings.Join(problems, "; "))
	}

	return nil
}

// Reason is a selection error without its sentinel prefix, for showing under a form field.
func Reason(err error) string {
	return strings.TrimPrefix(err.Error(), ErrInvalidSelection.Error()+": ")
}

// Preview counts the files a project with keep would get, without writing anything.
func (m *Manifest) Preview(keep Set, files []string) Report {
	var r Report

	for _, name := range files {
		if m.Keeps(keep, name) {
			r.Kept++
		} else {
			r.Removed++
		}
	}

	return r
}
