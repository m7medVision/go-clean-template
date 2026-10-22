package main

import (
	"context"
	"fmt"
	"slices"
	"strings"

	huh "charm.land/huh/v2"
	"github.com/m7medvision/go-clean-template/internal/scaffold"
)

// fieldError shows a selection error under the form field without its sentinel prefix.
type fieldError struct{ err error }

func (e fieldError) Error() string { return scaffold.Reason(e.err) }

// ask fills cmd from the interactive form: name, module path, preset, features, then a summary to
// confirm. It returns false when the user declines the summary.
func ask(ctx context.Context, cmd *scaffold.Command) (bool, error) {
	root, err := scaffold.FindRoot(".")
	if err != nil {
		return false, err
	}

	m, err := scaffold.Load(root + "/" + scaffold.ManifestPath)
	if err != nil {
		return false, err
	}

	preset := "fullstack"

	if err := basics(m, cmd, &preset).RunWithContext(ctx); err != nil {
		return false, fmt.Errorf("create - form: %w", err)
	}

	chosen, err := m.PresetFeatures(preset)
	if err != nil {
		return false, err
	}

	if err := features(m, chosen, &chosen).RunWithContext(ctx); err != nil {
		return false, fmt.Errorf("create - form: %w", err)
	}

	if err := applyChoices(cmd, m, preset, chosen); err != nil {
		return false, err
	}

	return confirm(ctx, root, m, cmd)
}

func basics(m *scaffold.Manifest, cmd *scaffold.Command, preset *string) *huh.Form {
	presets := make([]huh.Option[string], 0, len(m.Presets)+1)
	for _, p := range m.Presets {
		presets = append(presets, huh.NewOption(p.Label, p.ID))
	}

	presets = append(presets, huh.NewOption("Custom: pick every feature", scaffold.CustomPreset))

	return huh.NewForm(huh.NewGroup(
		huh.NewInput().Title("Project name").
			Description("Its directory under scaffold/out; empty names it after the preset.").
			Value(&cmd.Name).
			Validate(func(s string) error {
				if strings.ContainsAny(s, `/\ `) {
					return fieldError{fmt.Errorf("%w: a name, not a path", scaffold.ErrInvalidSelection)}
				}

				return nil
			}),
		huh.NewInput().Title("Go module path").
			Description("Empty keeps "+m.Module+".").
			Placeholder("github.com/you/service").
			Value(&cmd.Module),
		huh.NewSelect[string]().Title("Start from a preset").Options(presets...).Value(preset),
	))
}

// features is the checkbox list, pre-checked from the preset. A selection that breaks a
// requirement cannot be submitted.
func features(m *scaffold.Manifest, preselected []string, chosen *[]string) *huh.Form {
	options := make([]huh.Option[string], 0, len(m.Choices()))
	for _, c := range m.Choices() {
		options = append(options, huh.NewOption(c.Label, c.ID).Selected(slices.Contains(preselected, c.ID)))
	}

	return huh.NewForm(huh.NewGroup(
		huh.NewMultiSelect[string]().Title("Features").
			Description("REST is always included. space: toggle · enter: next").
			Options(options...).
			Value(chosen).
			Validate(func(ids []string) error {
				if err := m.ValidateChoices(ids); err != nil {
					return fieldError{err}
				}

				return nil
			}),
	))
}

// applyChoices turns the form's answers into the command's selection: the preset as is when its features
// were left unchanged, otherwise a custom pick.
func applyChoices(cmd *scaffold.Command, m *scaffold.Manifest, preset string, chosen []string) error {
	presetFeatures, err := m.PresetFeatures(preset)
	if err != nil {
		return err
	}

	if slices.Equal(sorted(presetFeatures), sorted(chosen)) {
		cmd.Preset = preset

		return nil
	}

	cmd.Preset, cmd.With = scaffold.CustomPreset, chosen

	return nil
}

func sorted(ids []string) []string {
	ids = slices.Clone(ids)
	slices.Sort(ids)

	return ids
}

// confirm shows what will be written and asks whether to go ahead and whether to run the check.
func confirm(ctx context.Context, root string, m *scaffold.Manifest, cmd *scaffold.Command) (bool, error) {
	keep, err := m.Resolve(scaffold.Selection{Preset: cmd.Preset, With: cmd.With})
	if err != nil {
		return false, err
	}

	files, err := scaffold.ListFiles(ctx, root)
	if err != nil {
		return false, err
	}

	report := m.Preview(keep, files)
	ok := true

	summary := fmt.Sprintf("Output: %s\nFeatures: %s\nFiles kept: %d   removed: %d",
		scaffold.Display(root, cmd.OutDir(root)), strings.Join(keep.IDs(), ", "), report.Kept, report.Removed)

	err = huh.NewForm(huh.NewGroup(
		huh.NewNote().Title("Summary").Description(summary),
		huh.NewConfirm().Title("Run make pre-commit in it afterwards?").
			Description("Proves the project builds, lints and passes its tests. Takes a few minutes.").
			Value(&cmd.Check),
		huh.NewConfirm().Title("Generate it?").Affirmative("Generate").Negative("Back out").Value(&ok),
	)).RunWithContext(ctx)
	if err != nil {
		return false, fmt.Errorf("create - form: %w", err)
	}

	return ok, nil
}
