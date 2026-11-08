package behavior_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/netip"
	"testing"
	"time"

	"github.com/m7medvision/go-clean-template/internal/entity"
	"github.com/m7medvision/go-clean-template/internal/repo/repomock"
	"github.com/m7medvision/go-clean-template/internal/usecase/behavior"
	"github.com/m7medvision/go-clean-template/pkg/cqrs"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

const (
	_tick    = time.Millisecond
	_timeout = 50 * time.Millisecond
)

var (
	errInvalid    = errors.New("title is empty")
	errCheckpoint = errors.New("checkpoint read failed")
	errStore      = errors.New("read model unreachable")
	errValkey     = errors.New("valkey unreachable")
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

// The event types the tests' writes append.
const (
	_thingWritten = "ThingWritten"
	_thingEdited  = "ThingEdited"
)

// task projects ThingWritten, as the task projector does the task events.
func task() map[string][]string {
	return map[string][]string{_thingWritten: {"task"}}
}

// writes answers a write of one ThingWritten at position 7.
func writes(context.Context, WriteThing) (entity.Written, error) {
	return entity.Written{ID: "t1", Position: 7, Events: []string{_thingWritten}}, nil
}

// wroteNothing answers a write that appended no events, as a sign-in to an account that exists does.
func wroteNothing(context.Context, WriteThing) (entity.Written, error) {
	return entity.Written{ID: "t1"}, nil
}

func TestWaitReturnsOnceTheProjectorsReachTheWrite(t *testing.T) {
	t.Parallel()

	cp := &fakeCheckpoints{at: 7, behind: 2}

	written, err := sendWrite(t, []cqrs.Step{behavior.Wait(cp, task(), time.Second, _tick)},
		writes, WriteThing{Title: "a"})

	require.NoError(t, err)
	assert.Equal(t, int64(7), written.Position)
	assert.Equal(t, [][]string{{"task"}, {"task"}, {"task"}}, cp.asked, "asked until the projector reached 7, then stopped")
}

func TestWaitGivesUpWithErrReadModelBehind(t *testing.T) {
	t.Parallel()

	_, err := sendWrite(t, []cqrs.Step{behavior.Wait(&fakeCheckpoints{}, task(), _timeout, _tick)},
		writes, WriteThing{Title: "a"})

	require.ErrorIs(t, err, entity.ErrReadModelBehind)
}

func TestWaitReturnsACheckpointFailure(t *testing.T) {
	t.Parallel()

	cp := &fakeCheckpoints{err: errCheckpoint}

	_, err := sendWrite(t, []cqrs.Step{behavior.Wait(cp, task(), _timeout, _tick)},
		writes, WriteThing{Title: "a"})

	require.ErrorIs(t, err, errCheckpoint)
}

func TestWaitSkipsWhatWroteNothingAndWhenNothingIsProjected(t *testing.T) {
	t.Parallel()

	cp := &fakeCheckpoints{}

	_, err := sendWrite(t, []cqrs.Step{behavior.Wait(cp, task(), _timeout, _tick)}, wroteNothing, WriteThing{Title: "a"})
	require.NoError(t, err)

	_, err = sendWrite(t, []cqrs.Step{behavior.Wait(cp, nil, _timeout, _tick)}, writes, WriteThing{Title: "a"})
	require.NoError(t, err)

	assert.Empty(t, cp.asked)
}

// A projector that handles none of the events a write appended is not waited for, so a stuck one
// holds up only the writes it projects; one that handles several of them is asked for once.
func TestWaitWaitsOnlyForTheProjectorsOfWhatWasWritten(t *testing.T) {
	t.Parallel()

	cp := &fakeCheckpoints{at: 7}
	projectors := map[string][]string{_thingWritten: {"task"}, _thingEdited: {"task", "audit"}, "OtherWritten": {"stuck"}}
	writesAndEdits := func(context.Context, WriteThing) (entity.Written, error) {
		return entity.Written{ID: "t1", Position: 7, Events: []string{_thingWritten, _thingEdited}}, nil
	}

	_, err := sendWrite(t, []cqrs.Step{behavior.Wait(cp, projectors, time.Second, _tick)},
		writesAndEdits, WriteThing{Title: "a"})

	require.NoError(t, err)
	assert.Equal(t, [][]string{{"task", "audit"}}, cp.asked)
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
			return signedIn{ID: "u1", Position: 7, Events: []string{_thingWritten}, Token: "t"}, nil
		}))

	_, err := cqrs.SendCommand(context.Background(), cmds, SignInThing{})

	require.ErrorIs(t, err, entity.ErrReadModelBehind)
}

const (
	_scope        = "sign-in"
	_email        = "jo@example.com"
	_token        = "token"
	_limitTimeout = 500 * time.Millisecond
)

type LogInThing struct {
	cqrs.Query[string]

	Email string
}

func (LogInThing) LimitScope() string     { return _scope }
func (q LogInThing) LimitAccount() string { return q.Email }

func client() netip.Addr { return netip.MustParseAddr("203.0.113.7") }

func fromClient(t *testing.T) context.Context {
	t.Helper()

	return entity.ContextWithClient(t.Context(), client())
}

func allowed() entity.Allowance { return entity.Allowance{Allowed: true} }

type limitRun struct {
	limiter *repomock.MockLimiter
	logs    bytes.Buffer
	calls   int
}

func newLimitRun(t *testing.T) *limitRun {
	t.Helper()

	return &limitRun{limiter: repomock.NewMockLimiter(gomock.NewController(t))}
}

func (r *limitRun) send(ctx context.Context, body any, answer error) (any, error) {
	step := behavior.Limit(r.limiter, slog.New(slog.NewJSONHandler(&r.logs, nil)))
	next := func(context.Context) (any, error) {
		r.calls++

		return _token, answer
	}

	return step(ctx, cqrs.Message{Kind: cqrs.KindQuery, Name: "LogInThing", Body: body}, next)
}

func (r *limitRun) lines(t *testing.T) []map[string]any {
	t.Helper()

	var lines []map[string]any

	for dec := json.NewDecoder(&r.logs); dec.More(); {
		var line map[string]any

		require.NoError(t, dec.Decode(&line))

		lines = append(lines, line)
	}

	return lines
}

func TestLimitRefusesASpentBudgetBeforeTheHandler(t *testing.T) {
	t.Parallel()

	for name, tc := range map[string]struct {
		expect     func(l *repomock.MockLimiterMockRecorder)
		retryAfter time.Duration
	}{
		"the client's": {
			expect: func(l *repomock.MockLimiterMockRecorder) {
				l.TakeClient(gomock.Any(), _scope, client()).Return(entity.Allowance{RetryAfter: 30 * time.Second}, nil)
			},
			retryAfter: 30 * time.Second,
		},
		"the account's": {
			expect: func(l *repomock.MockLimiterMockRecorder) {
				l.TakeClient(gomock.Any(), _scope, client()).Return(allowed(), nil)
				l.CheckAccount(gomock.Any(), _scope, _email).Return(entity.Allowance{RetryAfter: 15 * time.Minute}, nil)
			},
			retryAfter: 15 * time.Minute,
		},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			r := newLimitRun(t)
			tc.expect(r.limiter.EXPECT())

			out, err := r.send(fromClient(t), LogInThing{Email: _email}, nil)

			require.ErrorIs(t, err, entity.ErrTooManyAttempts)

			var retry *entity.RetryAfterError

			require.ErrorAs(t, err, &retry)
			assert.Equal(t, tc.retryAfter, retry.RetryAfter)
			assert.EqualError(t, err, "behavior - Limit LogInThing: too many attempts")
			assert.Nil(t, out)
			assert.Zero(t, r.calls)
			assert.Empty(t, r.lines(t))
		})
	}
}

func TestLimitSettlesTheAccountByTheHandlersAnswer(t *testing.T) {
	t.Parallel()

	for name, tc := range map[string]struct {
		answer error
		expect func(l *repomock.MockLimiterMockRecorder)
	}{
		"a wrong password counts a failure": {
			answer: entity.ErrInvalidCredentials,
			expect: func(l *repomock.MockLimiterMockRecorder) {
				l.TakeAccountFailure(gomock.Any(), _scope, _email).Return(nil)
			},
		},
		"a success clears the account": {
			expect: func(l *repomock.MockLimiterMockRecorder) {
				l.ClearAccount(gomock.Any(), _scope, _email).Return(nil)
			},
		},
		"another failure neither counts nor clears": {
			answer: errStore,
			expect: func(*repomock.MockLimiterMockRecorder) {},
		},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			r := newLimitRun(t)
			r.limiter.EXPECT().TakeClient(gomock.Any(), _scope, client()).Return(allowed(), nil)
			r.limiter.EXPECT().CheckAccount(gomock.Any(), _scope, _email).Return(allowed(), nil)
			tc.expect(r.limiter.EXPECT())

			out, err := r.send(fromClient(t), LogInThing{Email: _email}, tc.answer)

			assert.Equal(t, tc.answer, err, "the handler's error, unwrapped")
			assert.Equal(t, _token, out)
			assert.Equal(t, 1, r.calls)
			assert.Empty(t, r.lines(t))
		})
	}
}

func TestLimitWithoutAClientSkipsTheClientBudget(t *testing.T) {
	t.Parallel()

	r := newLimitRun(t)
	r.limiter.EXPECT().CheckAccount(gomock.Any(), _scope, _email).Return(allowed(), nil)
	r.limiter.EXPECT().ClearAccount(gomock.Any(), _scope, _email).Return(nil)

	out, err := r.send(t.Context(), LogInThing{Email: _email}, nil)

	require.NoError(t, err)
	assert.Equal(t, _token, out)
	assert.Equal(t, 1, r.calls)
}

func TestLimitCountsTheFailureOfARequestWhoseCallerWentAway(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(fromClient(t))
	cancel()

	live := func(ctx context.Context) { assert.NoError(t, ctx.Err()) }

	r := newLimitRun(t)
	r.limiter.EXPECT().TakeClient(gomock.Any(), _scope, client()).
		DoAndReturn(func(ctx context.Context, _ string, _ netip.Addr) (entity.Allowance, error) {
			live(ctx)

			return allowed(), nil
		})
	r.limiter.EXPECT().CheckAccount(gomock.Any(), _scope, _email).
		DoAndReturn(func(ctx context.Context, _, _ string) (entity.Allowance, error) {
			live(ctx)

			return allowed(), nil
		})
	r.limiter.EXPECT().TakeAccountFailure(gomock.Any(), _scope, _email).
		DoAndReturn(func(ctx context.Context, _, _ string) error {
			live(ctx)

			return nil
		})

	_, err := r.send(ctx, LogInThing{Email: _email}, entity.ErrInvalidCredentials)

	require.ErrorIs(t, err, entity.ErrInvalidCredentials)
	assert.Empty(t, r.lines(t))
}

func TestLimitWithoutAnAccountMakesNoAccountCalls(t *testing.T) {
	t.Parallel()

	for name, answer := range map[string]error{
		"a success":        nil,
		"a wrong password": entity.ErrInvalidCredentials,
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			r := newLimitRun(t)
			r.limiter.EXPECT().TakeClient(gomock.Any(), _scope, client()).Return(allowed(), nil)

			out, err := r.send(fromClient(t), LogInThing{}, answer)

			assert.Equal(t, answer, err)
			assert.Equal(t, _token, out)
			assert.Equal(t, 1, r.calls)
		})
	}
}

func TestLimitPassesAMessageThatIsNotLimited(t *testing.T) {
	t.Parallel()

	r := newLimitRun(t)

	out, err := r.send(fromClient(t), WriteThing{Title: "a"}, nil)

	require.NoError(t, err)
	assert.Equal(t, _token, out)
	assert.Equal(t, 1, r.calls)
}

func TestLimitLetsTheAttemptThroughWhenTheLimiterFails(t *testing.T) {
	t.Parallel()

	for name, tc := range map[string]struct {
		answer error
		expect func(l *repomock.MockLimiterMockRecorder)
	}{
		"taking from the client": {
			answer: entity.ErrInvalidCredentials,
			expect: func(l *repomock.MockLimiterMockRecorder) {
				l.TakeClient(gomock.Any(), _scope, client()).Return(entity.Allowance{}, errValkey)
			},
		},
		"checking the account": {
			answer: entity.ErrInvalidCredentials,
			expect: func(l *repomock.MockLimiterMockRecorder) {
				l.TakeClient(gomock.Any(), _scope, client()).Return(allowed(), nil)
				l.CheckAccount(gomock.Any(), _scope, _email).Return(entity.Allowance{}, errValkey)
			},
		},
		"counting a failure": {
			answer: entity.ErrInvalidCredentials,
			expect: func(l *repomock.MockLimiterMockRecorder) {
				l.TakeClient(gomock.Any(), _scope, client()).Return(allowed(), nil)
				l.CheckAccount(gomock.Any(), _scope, _email).Return(allowed(), nil)
				l.TakeAccountFailure(gomock.Any(), _scope, _email).Return(errValkey)
			},
		},
		"clearing the account": {
			expect: func(l *repomock.MockLimiterMockRecorder) {
				l.TakeClient(gomock.Any(), _scope, client()).Return(allowed(), nil)
				l.CheckAccount(gomock.Any(), _scope, _email).Return(allowed(), nil)
				l.ClearAccount(gomock.Any(), _scope, _email).Return(errValkey)
			},
		},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			r := newLimitRun(t)
			tc.expect(r.limiter.EXPECT())

			out, err := r.send(fromClient(t), LogInThing{Email: _email}, tc.answer)

			assert.Equal(t, tc.answer, err)
			assert.Equal(t, _token, out)
			assert.Equal(t, 1, r.calls)

			lines := r.lines(t)
			require.Len(t, lines, 1, "one warning for the attempt")
			assert.Equal(t, "WARN", lines[0]["level"])
			assert.Equal(t, "sign-in limit unavailable", lines[0]["msg"])
			assert.Equal(t, _scope, lines[0]["scope"])
			assert.Equal(t, errValkey.Error(), lines[0]["error"])
		})
	}
}

func TestLimitGivesEveryLimiterCallADeadline(t *testing.T) {
	t.Parallel()

	for name, answer := range map[string]error{
		"a wrong password": entity.ErrInvalidCredentials,
		"a success":        nil,
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			r := newLimitRun(t)
			bounded := func(ctx context.Context) {
				deadline, ok := ctx.Deadline()
				if assert.True(t, ok, "the call has a deadline") {
					assert.LessOrEqual(t, time.Until(deadline), _limitTimeout)
				}
			}

			r.limiter.EXPECT().TakeClient(gomock.Any(), _scope, client()).
				DoAndReturn(func(ctx context.Context, _ string, _ netip.Addr) (entity.Allowance, error) {
					bounded(ctx)

					return allowed(), nil
				})
			r.limiter.EXPECT().CheckAccount(gomock.Any(), _scope, _email).
				DoAndReturn(func(ctx context.Context, _, _ string) (entity.Allowance, error) {
					bounded(ctx)

					return allowed(), nil
				})

			settle := func(ctx context.Context, _, _ string) error {
				bounded(ctx)

				return nil
			}
			if answer == nil {
				r.limiter.EXPECT().ClearAccount(gomock.Any(), _scope, _email).DoAndReturn(settle)
			} else {
				r.limiter.EXPECT().TakeAccountFailure(gomock.Any(), _scope, _email).DoAndReturn(settle)
			}

			_, err := r.send(fromClient(t), LogInThing{Email: _email}, answer)

			assert.Equal(t, answer, err)
		})
	}
}
