package behavior_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/m7medvision/go-clean-template/internal/entity"
	"github.com/m7medvision/go-clean-template/internal/repo"
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

type WriteThing struct {
	cqrs.Command[entity.Written]

	UserID string
	Title  string
}

func (c WriteThing) ActorID() string { return c.UserID }

func (c WriteThing) Validate() error {
	if c.Title == "" {
		return errInvalid
	}

	return nil
}

type SignInThing struct {
	cqrs.Command[signedIn]
}

type ListEverything struct {
	cqrs.Query[[]string]

	By entity.Principal
}

func (ListEverything) AdminOnly()                 {}
func (q ListEverything) Caller() entity.Principal { return q.By }

// fakeCheckpoints reports the projectors at position 0 for the first behind asks, and at
// position at from then on, so a test decides when they catch up without a clock.
type fakeCheckpoints struct {
	at     int64
	behind int
	err    error
	asked  [][]string
}

func (f *fakeCheckpoints) Reached(_ context.Context, names []string, position int64) (bool, error) {
	f.asked = append(f.asked, names)

	var at int64
	if len(f.asked) > f.behind {
		at = f.at
	}

	return at >= position, f.err
}

func sendWrite(t *testing.T, steps []cqrs.Step, h cqrs.HandlerFunc[WriteThing, entity.Written], cmd WriteThing) (entity.Written, error) {
	t.Helper()

	cmds := cqrs.NewCommands(steps...)
	cqrs.HandleCommand(cmds, h)

	return cqrs.SendCommand(context.Background(), cmds, cmd)
}

type thingWritten struct{}

func (thingWritten) EventType() string  { return "ThingWritten" }
func (thingWritten) SchemaVersion() int { return 1 }

type otherWritten struct{}

func (otherWritten) EventType() string  { return "OtherWritten" }
func (otherWritten) SchemaVersion() int { return 1 }

// projector is a subscriber the wait step can wait on, handling the given events.
type projector struct {
	name    string
	handles []entity.Event
}

func (p projector) Name() string                                      { return p.name }
func (p projector) Handles() []entity.Event                           { return p.handles }
func (projector) Handle(context.Context, *entity.RecordedEvent) error { return nil }

// task projects ThingWritten, as the task projector does the task events.
func task() []repo.Subscriber {
	return []repo.Subscriber{projector{name: "task", handles: []entity.Event{thingWritten{}}}}
}

// writes answers a write of one ThingWritten at position.
func writes(position int64) cqrs.HandlerFunc[WriteThing, entity.Written] {
	return func(context.Context, WriteThing) (entity.Written, error) {
		return entity.Written{ID: "t1", Position: position, Events: []string{thingWritten{}.EventType()}}, nil
	}
}

func TestWaitReturnsOnceTheProjectorsReachTheWrite(t *testing.T) {
	t.Parallel()

	cp := &fakeCheckpoints{at: 7, behind: 2}

	written, err := sendWrite(t, []cqrs.Step{behavior.Wait(cp, task(), time.Second, _tick)},
		writes(7), WriteThing{Title: "a"})

	require.NoError(t, err)
	assert.Equal(t, int64(7), written.Position)
	assert.Equal(t, [][]string{{"task"}, {"task"}, {"task"}}, cp.asked, "asked until the projector reached 7, then stopped")
}

func TestWaitGivesUpWithErrReadModelBehind(t *testing.T) {
	t.Parallel()

	_, err := sendWrite(t, []cqrs.Step{behavior.Wait(&fakeCheckpoints{}, task(), _timeout, _tick)},
		writes(7), WriteThing{Title: "a"})

	require.ErrorIs(t, err, entity.ErrReadModelBehind)
}

func TestWaitReturnsACheckpointFailure(t *testing.T) {
	t.Parallel()

	cp := &fakeCheckpoints{err: errCheckpoint}

	_, err := sendWrite(t, []cqrs.Step{behavior.Wait(cp, task(), _timeout, _tick)},
		writes(7), WriteThing{Title: "a"})

	require.ErrorIs(t, err, errCheckpoint)
}

func TestWaitSkipsWhatWroteNothingAndWhenNothingIsProjected(t *testing.T) {
	t.Parallel()

	cp := &fakeCheckpoints{}

	_, err := sendWrite(t, []cqrs.Step{behavior.Wait(cp, task(), _timeout, _tick)},
		writes(0), WriteThing{Title: "a"})
	require.NoError(t, err)

	_, err = sendWrite(t, []cqrs.Step{behavior.Wait(cp, nil, _timeout, _tick)}, writes(7), WriteThing{Title: "a"})
	require.NoError(t, err)

	assert.Empty(t, cp.asked)
}

// A projector that handles none of the events a write appended is not waited for, so a stuck one
// holds up only the writes it projects.
func TestWaitWaitsOnlyForTheProjectorsOfWhatWasWritten(t *testing.T) {
	t.Parallel()

	cp := &fakeCheckpoints{at: 7}
	projectors := []repo.Subscriber{
		projector{name: "task", handles: []entity.Event{thingWritten{}}},
		projector{name: "stuck", handles: []entity.Event{otherWritten{}}},
	}

	_, err := sendWrite(t, []cqrs.Step{behavior.Wait(cp, projectors, time.Second, _tick)},
		writes(7), WriteThing{Title: "a"})

	require.NoError(t, err)
	assert.Equal(t, [][]string{{"task"}}, cp.asked)
}

func TestRetryRunsTheHandlerAgainOnAConflict(t *testing.T) {
	t.Parallel()

	calls := 0
	h := func(context.Context, WriteThing) (entity.Written, error) {
		calls++
		if calls < 3 {
			return entity.Written{}, entity.ErrConcurrencyConflict
		}

		return entity.Written{ID: "t1"}, nil
	}

	written, err := sendWrite(t, []cqrs.Step{behavior.Retry(3)}, h, WriteThing{Title: "a"})

	require.NoError(t, err)
	assert.Equal(t, "t1", written.ID)
	assert.Equal(t, 3, calls)
}

func TestRetryGivesUpAfterItsAttempts(t *testing.T) {
	t.Parallel()

	calls := 0
	h := func(context.Context, WriteThing) (entity.Written, error) {
		calls++

		return entity.Written{}, entity.ErrConcurrencyConflict
	}

	_, err := sendWrite(t, []cqrs.Step{behavior.Retry(3)}, h, WriteThing{Title: "a"})

	require.ErrorIs(t, err, entity.ErrConcurrencyConflict)
	assert.Equal(t, 3, calls)
}

func TestRetryLeavesOtherFailuresAlone(t *testing.T) {
	t.Parallel()

	calls := 0
	h := func(context.Context, WriteThing) (entity.Written, error) {
		calls++

		return entity.Written{}, entity.ErrTaskNotFound
	}

	_, err := sendWrite(t, []cqrs.Step{behavior.Retry(3)}, h, WriteThing{Title: "a"})

	require.ErrorIs(t, err, entity.ErrTaskNotFound)
	assert.Equal(t, 1, calls)
}

func TestAuthorizeStopsACallerWithoutTheRole(t *testing.T) {
	t.Parallel()

	qs := cqrs.NewQueries(behavior.Authorize())
	cqrs.HandleQuery(qs, cqrs.HandlerFunc[ListEverything, []string](
		func(context.Context, ListEverything) ([]string, error) { return []string{"all"}, nil },
	))

	_, err := cqrs.SendQuery(context.Background(), qs,
		ListEverything{By: entity.Principal{UserID: "u1", Roles: []entity.Role{entity.RoleUser}}})
	require.ErrorIs(t, err, entity.ErrAdminRequired)

	got, err := cqrs.SendQuery(context.Background(), qs,
		ListEverything{By: entity.Principal{UserID: "a1", Roles: []entity.Role{entity.RoleAdmin}}})
	require.NoError(t, err)
	assert.Equal(t, []string{"all"}, got)
}

func TestValidateStopsAnInvalidMessage(t *testing.T) {
	t.Parallel()

	called := false
	h := func(context.Context, WriteThing) (entity.Written, error) {
		called = true

		return entity.Written{}, nil
	}

	_, err := sendWrite(t, []cqrs.Step{behavior.Validate()}, h, WriteThing{})

	require.ErrorIs(t, err, errInvalid)
	assert.False(t, called)
}

func TestStampPutsTheActorAndTheMessageInTheMetadata(t *testing.T) {
	t.Parallel()

	var md entity.Metadata

	h := func(ctx context.Context, _ WriteThing) (entity.Written, error) {
		md = entity.MetadataFromContext(ctx)

		return entity.Written{}, nil
	}

	_, err := sendWrite(t, []cqrs.Step{behavior.Stamp()}, h, WriteThing{UserID: "u1", Title: "a"})

	require.NoError(t, err)
	assert.Equal(t, entity.Metadata{UserID: "u1", CausationID: "command WriteThing"}, md)
}

// signedIn is a command result that carries a token beside the write, as a sign-in does.
type signedIn struct {
	entity.Written

	Token string
}

func TestWaitAlsoWaitsForAResultThatEmbedsWritten(t *testing.T) {
	t.Parallel()

	cmds := cqrs.NewCommands(behavior.Wait(&fakeCheckpoints{}, task(), _timeout, _tick))
	cqrs.HandleCommand(cmds, cqrs.HandlerFunc[SignInThing, signedIn](
		func(context.Context, SignInThing) (signedIn, error) {
			return signedIn{ID: "u1", Position: 7, Events: []string{thingWritten{}.EventType()}, Token: "t"}, nil
		}))

	_, err := cqrs.SendCommand(context.Background(), cmds, SignInThing{})

	require.ErrorIs(t, err, entity.ErrReadModelBehind)
}
