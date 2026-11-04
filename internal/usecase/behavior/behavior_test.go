package behavior_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/m7medvision/go-clean-template/internal/entity"
	"github.com/m7medvision/go-clean-template/internal/usecase/behavior"
	"github.com/m7medvision/go-clean-template/pkg/cqrs"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	_tick    = time.Millisecond
	_timeout = 50 * time.Millisecond
)

var (
	errInvalid    = errors.New("title is empty")
	errCheckpoint = errors.New("checkpoint read failed")
)

type writeThing struct {
	UserID string
	Title  string
}

func (writeThing) CommandName() string { return "WriteThing" }
func (c writeThing) ActorID() string   { return c.UserID }

func (c writeThing) Validate() error {
	if c.Title == "" {
		return errInvalid
	}

	return nil
}

type listEverything struct {
	By entity.Principal
}

func (listEverything) QueryName() string          { return "ListEverything" }
func (listEverything) RequiredRole() entity.Role  { return entity.RoleAdmin }
func (q listEverything) Caller() entity.Principal { return q.By }

// fakeCheckpoints reports the projectors as caught up once position reaches at.
type fakeCheckpoints struct {
	mu    sync.Mutex
	at    int64
	err   error
	asked [][]string
}

func (f *fakeCheckpoints) Reached(_ context.Context, names []string, position int64) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.asked = append(f.asked, names)

	return f.at >= position, f.err
}

func (f *fakeCheckpoints) catchUp(p int64) {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.at = p
}

func sendWrite(t *testing.T, steps []cqrs.Step, h cqrs.HandlerFunc[writeThing, cqrs.Written], cmd writeThing) (cqrs.Written, error) {
	t.Helper()

	cmds := cqrs.NewCommands(steps...)
	cqrs.HandleCommand(cmds, h)

	return cqrs.SendCommand[cqrs.Written](context.Background(), cmds, cmd)
}

func writes(position int64) cqrs.HandlerFunc[writeThing, cqrs.Written] {
	return func(context.Context, writeThing) (cqrs.Written, error) {
		return cqrs.Written{ID: "t1", Position: position}, nil
	}
}

func TestWaitReturnsOnceTheProjectorsReachTheWrite(t *testing.T) {
	t.Parallel()

	cp := &fakeCheckpoints{}

	time.AfterFunc(5*_tick, func() { cp.catchUp(7) })

	written, err := sendWrite(t, []cqrs.Step{behavior.Wait(cp, []string{"task"}, time.Second, _tick)},
		writes(7), writeThing{Title: "a"})

	require.NoError(t, err)
	assert.Equal(t, int64(7), written.Position)
	assert.Equal(t, []string{"task"}, cp.asked[len(cp.asked)-1])
}

func TestWaitGivesUpWithErrReadModelBehind(t *testing.T) {
	t.Parallel()

	_, err := sendWrite(t, []cqrs.Step{behavior.Wait(&fakeCheckpoints{}, []string{"task"}, _timeout, _tick)},
		writes(7), writeThing{Title: "a"})

	require.ErrorIs(t, err, entity.ErrReadModelBehind)
}

func TestWaitReturnsACheckpointFailure(t *testing.T) {
	t.Parallel()

	cp := &fakeCheckpoints{err: errCheckpoint}

	_, err := sendWrite(t, []cqrs.Step{behavior.Wait(cp, []string{"task"}, _timeout, _tick)},
		writes(7), writeThing{Title: "a"})

	require.ErrorIs(t, err, errCheckpoint)
}

func TestWaitSkipsWhatWroteNothingAndWhenNothingIsProjected(t *testing.T) {
	t.Parallel()

	cp := &fakeCheckpoints{}

	_, err := sendWrite(t, []cqrs.Step{behavior.Wait(cp, []string{"task"}, _timeout, _tick)},
		writes(0), writeThing{Title: "a"})
	require.NoError(t, err)

	_, err = sendWrite(t, []cqrs.Step{behavior.Wait(cp, nil, _timeout, _tick)}, writes(7), writeThing{Title: "a"})
	require.NoError(t, err)

	assert.Empty(t, cp.asked)
}

func TestRetryRunsTheHandlerAgainOnAConflict(t *testing.T) {
	t.Parallel()

	calls := 0
	h := func(context.Context, writeThing) (cqrs.Written, error) {
		calls++
		if calls < 3 {
			return cqrs.Written{}, entity.ErrConcurrencyConflict
		}

		return cqrs.Written{ID: "t1"}, nil
	}

	written, err := sendWrite(t, []cqrs.Step{behavior.Retry(3)}, h, writeThing{Title: "a"})

	require.NoError(t, err)
	assert.Equal(t, "t1", written.ID)
	assert.Equal(t, 3, calls)
}

func TestRetryGivesUpAfterItsAttempts(t *testing.T) {
	t.Parallel()

	calls := 0
	h := func(context.Context, writeThing) (cqrs.Written, error) {
		calls++

		return cqrs.Written{}, entity.ErrConcurrencyConflict
	}

	_, err := sendWrite(t, []cqrs.Step{behavior.Retry(3)}, h, writeThing{Title: "a"})

	require.ErrorIs(t, err, entity.ErrConcurrencyConflict)
	assert.Equal(t, 3, calls)
}

func TestRetryLeavesOtherFailuresAlone(t *testing.T) {
	t.Parallel()

	calls := 0
	h := func(context.Context, writeThing) (cqrs.Written, error) {
		calls++

		return cqrs.Written{}, entity.ErrTaskNotFound
	}

	_, err := sendWrite(t, []cqrs.Step{behavior.Retry(3)}, h, writeThing{Title: "a"})

	require.ErrorIs(t, err, entity.ErrTaskNotFound)
	assert.Equal(t, 1, calls)
}

func TestAuthorizeStopsACallerWithoutTheRole(t *testing.T) {
	t.Parallel()

	qs := cqrs.NewQueries(behavior.Authorize())
	cqrs.HandleQuery(qs, cqrs.HandlerFunc[listEverything, []string](
		func(context.Context, listEverything) ([]string, error) { return []string{"all"}, nil },
	))

	_, err := cqrs.SendQuery[[]string](context.Background(), qs,
		listEverything{By: entity.Principal{UserID: "u1", Roles: []entity.Role{entity.RoleUser}}})
	require.ErrorIs(t, err, entity.ErrAdminRequired)

	got, err := cqrs.SendQuery[[]string](context.Background(), qs,
		listEverything{By: entity.Principal{UserID: "a1", Roles: []entity.Role{entity.RoleAdmin}}})
	require.NoError(t, err)
	assert.Equal(t, []string{"all"}, got)
}

func TestValidateStopsAnInvalidMessage(t *testing.T) {
	t.Parallel()

	called := false
	h := func(context.Context, writeThing) (cqrs.Written, error) {
		called = true

		return cqrs.Written{}, nil
	}

	_, err := sendWrite(t, []cqrs.Step{behavior.Validate()}, h, writeThing{})

	require.ErrorIs(t, err, errInvalid)
	assert.False(t, called)
}

func TestStampPutsTheActorAndTheMessageInTheMetadata(t *testing.T) {
	t.Parallel()

	var md entity.Metadata

	h := func(ctx context.Context, _ writeThing) (cqrs.Written, error) {
		md = entity.MetadataFromContext(ctx)

		return cqrs.Written{}, nil
	}

	_, err := sendWrite(t, []cqrs.Step{behavior.Stamp()}, h, writeThing{UserID: "u1", Title: "a"})

	require.NoError(t, err)
	assert.Equal(t, entity.Metadata{UserID: "u1", CausationID: "command WriteThing"}, md)
}

// signedIn is a command result that carries a token beside the write, as a sign-in does.
type signedIn struct {
	cqrs.Written

	Token string
}

func TestWaitAlsoWaitsForAResultThatEmbedsWritten(t *testing.T) {
	t.Parallel()

	cmds := cqrs.NewCommands(behavior.Wait(&fakeCheckpoints{}, []string{"task"}, _timeout, _tick))
	cqrs.HandleCommand(cmds, cqrs.HandlerFunc[writeThing, signedIn](
		func(context.Context, writeThing) (signedIn, error) {
			return signedIn{ID: "u1", Position: 7, Token: "t"}, nil
		}))

	_, err := cqrs.SendCommand[signedIn](context.Background(), cmds, writeThing{Title: "a"})

	require.ErrorIs(t, err, entity.ErrReadModelBehind)
}
