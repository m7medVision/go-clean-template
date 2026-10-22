package scaffold

import (
	"context"
	"fmt"
	"io"
	"os/exec"
)

// Runner runs one shell command in a directory.
type Runner interface {
	Run(ctx context.Context, dir, command string) error
}

// ShellRunner runs commands with sh, sending their output to Stdout and Stderr. Stdin, when set,
// is passed on (make dev attaches to tmux).
type ShellRunner struct {
	Stdin          io.Reader
	Stdout, Stderr io.Writer
}

// Run -.
func (r ShellRunner) Run(ctx context.Context, dir, command string) error {
	cmd := exec.CommandContext(ctx, "sh", "-c", command)
	cmd.Dir = dir
	cmd.Stdin = r.Stdin
	cmd.Stdout = r.Stdout
	cmd.Stderr = r.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("scaffold - ShellRunner - %s: %w", command, err)
	}

	return nil
}

// Regenerate runs the manifest's regen steps for the kept features, in order, inside dir. It stops
// at the first failing step.
func Regenerate(ctx context.Context, m *Manifest, keep Set, dir string, run Runner) error {
	for _, step := range m.Regen {
		if step.When != "" && !keep.Has(step.When) {
			continue
		}

		if err := run.Run(ctx, dir, step.Run); err != nil {
			return fmt.Errorf("scaffold - Regenerate - %q: %w", step.Run, err)
		}
	}

	return nil
}
