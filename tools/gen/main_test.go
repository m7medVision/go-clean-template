package main

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
		"unknown transport":          {"new-domain", "--name", "invoice", "--transports", "rest,soap"},
		"empty transport":            {"new-domain", "--name", "invoice", "--transports", "rest,"},
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

	for name, tc := range map[string]struct {
		dir  string
		args []string
		want string
	}{
		"new-domain": {
			dir:  sharedRepo(t),
			args: []string{"new-domain", "--name", "invoice", "--dry-run"},
			want: "43 files to create, 9 to edit",
		},
		"new-usecase": {
			dir:  gitRepo(t),
			args: []string{"new-usecase", "--domain", "task", "--name", "archive-task", "--kind", "command", "--dry-run"},
			want: "dry run",
		},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			var stdout, stderr bytes.Buffer

			before := fingerprint(t, tc.dir)

			assert.Equal(t, _exitOK, run(t.Context(), append(tc.args, "--root", tc.dir), &stdout, &stderr), stderr.String())
			assert.Contains(t, stdout.String(), tc.want)
			assert.Equal(t, before, fingerprint(t, tc.dir))
		})
	}
}

func TestParseTransportsKeepsTheirOrderOnceEach(t *testing.T) {
	t.Parallel()

	for value, want := range map[string][]string{
		"":                  {"rest", "grpc", "graphql"},
		"graphql,rest":      {"rest", "graphql"},
		"grpc, grpc":        {"grpc"},
		"rest,grpc,graphql": {"rest", "grpc", "graphql"},
	} {
		got, err := parseTransports(value, &bytes.Buffer{})
		require.NoError(t, err, value)
		assert.Equal(t, want, got, value)
	}
}
