package main

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRunAnswersUsageForABadCommandLine(t *testing.T) {
	t.Parallel()

	for name, args := range map[string][]string{
		"no command":                 nil,
		"unknown command":            {"new-thing"},
		"new-domain without a name":  {"new-domain"},
		"unknown flag":               {"new-domain", "--name", "invoice", "--shade", "red"},
		"new-usecase without a kind": {"new-usecase", "--domain", "task", "--name", "archive-task"},
		"new-usecase without domain": {"new-usecase", "--name", "archive-task", "--kind", "command"},
		"new-usecase unknown kind":   {"new-usecase", "--domain", "task", "--name", "archive-task", "--kind", "event"},
		"stray argument":             {"new-domain", "--name", "invoice", "extra"},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			var stdout, stderr bytes.Buffer

			assert.Equal(t, _exitUsage, run(t.Context(), args, &stdout, &stderr))
			assert.Contains(t, stderr.String(), "usage:")
			assert.Empty(t, stdout.String())
		})
	}
}

func TestRunDryRunPrintsThePlanAndWritesNothing(t *testing.T) {
	t.Parallel()

	for name, args := range map[string][]string{
		"new-domain":  {"new-domain", "--root", gitRepo(t), "--name", "invoice", "--dry-run"},
		"new-usecase": {"new-usecase", "--root", gitRepo(t), "--domain", "task", "--name", "archive-task", "--kind", "command", "--dry-run"},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			var stdout, stderr bytes.Buffer

			assert.Equal(t, _exitOK, run(t.Context(), args, &stdout, &stderr), stderr.String())
			assert.Contains(t, stdout.String(), "dry run")
		})
	}
}
