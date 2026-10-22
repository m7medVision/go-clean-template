package scaffold_test

import (
	"context"
	"errors"
	"testing"

	"github.com/m7medvision/go-clean-template/internal/scaffold"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const _regenManifest = `
features:
  - id: rest
    core: true
  - id: grpc
  - id: web
regen:
  - run: make sqlc
  - run: make proto-v1
    when: grpc
  - run: make web-api
    when: web
  - run: make format
`

var errStepFailed = errors.New("exit status 2")

type fakeRunner struct {
	ran    []string
	failOn string
}

func (f *fakeRunner) Run(_ context.Context, dir, command string) error {
	f.ran = append(f.ran, dir+": "+command)

	if command == f.failOn {
		return errStepFailed
	}

	return nil
}

func TestRegenerateRunsTheStepsOfKeptFeaturesInOrder(t *testing.T) {
	t.Parallel()

	m, err := scaffold.Parse([]byte(_regenManifest))
	require.NoError(t, err)

	keep, err := m.Resolve(scaffold.Selection{With: []string{"web"}})
	require.NoError(t, err)

	run := &fakeRunner{}
	require.NoError(t, scaffold.Regenerate(t.Context(), m, keep, "/out", run))

	assert.Equal(t, []string{"/out: make sqlc", "/out: make web-api", "/out: make format"}, run.ran)
}

func TestRegenerateStopsAtTheFirstFailingStep(t *testing.T) {
	t.Parallel()

	m, err := scaffold.Parse([]byte(_regenManifest))
	require.NoError(t, err)

	keep, err := m.Resolve(scaffold.Selection{})
	require.NoError(t, err)

	run := &fakeRunner{failOn: "make proto-v1"}
	err = scaffold.Regenerate(t.Context(), m, keep, "/out", run)

	require.ErrorIs(t, err, errStepFailed)
	assert.Contains(t, err.Error(), "make proto-v1")
	assert.Equal(t, []string{"/out: make sqlc", "/out: make proto-v1"}, run.ran)
}

func TestParseRejectsARegenStepForAnUnknownFeature(t *testing.T) {
	t.Parallel()

	_, err := scaffold.Parse([]byte("features:\n  - {id: rest}\nregen:\n  - {run: make x, when: nope}\n"))
	require.ErrorIs(t, err, scaffold.ErrInvalidManifest)
	assert.Contains(t, err.Error(), `regen step "make x" names unknown feature "nope"`)
}
