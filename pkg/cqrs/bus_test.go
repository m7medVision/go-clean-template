package cqrs_test

import (
	"context"
	"errors"
	"testing"

	"github.com/m7medvision/go-clean-template/pkg/cqrs"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var errHandler = errors.New("handler failed")

// written stands in for the app's result of a write.
type written struct {
	ID       string
	Position int64
}

type CreateThing struct {
	cqrs.Command[written]

	Title string
}

type getFailure struct {
	cqrs.Query[error]
}

type GetThing struct {
	cqrs.Query[thing]

	ID string
}

type thing struct {
	ID    string
	Title string
}

func TestSendCommandReturnsTheHandlersResult(t *testing.T) {
	t.Parallel()

	cmds := cqrs.NewCommands()
	cqrs.HandleCommand(cmds, cqrs.HandlerFunc[CreateThing, written](
		func(_ context.Context, cmd CreateThing) (written, error) {
			return written{ID: "thing-" + cmd.Title, Position: 7}, nil
		},
	))

	got, err := cqrs.SendCommand(context.Background(), cmds, CreateThing{Title: "a"})

	require.NoError(t, err)
	assert.Equal(t, written{ID: "thing-a", Position: 7}, got)
}

func TestSendQueryReturnsTheHandlersResult(t *testing.T) {
	t.Parallel()

	qs := cqrs.NewQueries()
	cqrs.HandleQuery(qs, cqrs.HandlerFunc[GetThing, thing](func(_ context.Context, q GetThing) (thing, error) {
		return thing{ID: q.ID, Title: "found"}, nil
	}))

	got, err := cqrs.SendQuery(context.Background(), qs, GetThing{ID: "t1"})

	require.NoError(t, err)
	assert.Equal(t, thing{ID: "t1", Title: "found"}, got)
}

func TestHandlerErrorIsReturnedWithAZeroResult(t *testing.T) {
	t.Parallel()

	qs := cqrs.NewQueries()
	cqrs.HandleQuery(qs, cqrs.HandlerFunc[GetThing, thing](func(context.Context, GetThing) (thing, error) {
		return thing{ID: "partial"}, errHandler
	}))

	got, err := cqrs.SendQuery(context.Background(), qs, GetThing{})

	require.ErrorIs(t, err, errHandler)
	assert.Equal(t, thing{}, got)
}

func TestSendWithoutAHandlerIsErrNoHandler(t *testing.T) {
	t.Parallel()

	_, err := cqrs.SendCommand(context.Background(), cqrs.NewCommands(), CreateThing{})

	require.ErrorIs(t, err, cqrs.ErrNoHandler)
	assert.Contains(t, err.Error(), "CreateThing")
}

func TestASecondHandlerForOneTypePanics(t *testing.T) {
	t.Parallel()

	cmds := cqrs.NewCommands()
	h := cqrs.HandlerFunc[CreateThing, written](func(context.Context, CreateThing) (written, error) {
		return written{}, nil
	})
	cqrs.HandleCommand(cmds, h)

	assert.Panics(t, func() { cqrs.HandleCommand(cmds, h) })
}

func TestAStepThatAnswersAnotherTypeIsErrResultType(t *testing.T) {
	t.Parallel()

	replace := func(context.Context, cqrs.Message, cqrs.Next) (any, error) { return "not a thing", nil }

	qs := cqrs.NewQueries(replace)
	cqrs.HandleQuery(qs, cqrs.HandlerFunc[GetThing, thing](func(context.Context, GetThing) (thing, error) {
		return thing{}, nil
	}))

	_, err := cqrs.SendQuery(context.Background(), qs, GetThing{})

	require.ErrorIs(t, err, cqrs.ErrResultType)
}

func TestANilInterfaceResultIsTheZeroValue(t *testing.T) {
	t.Parallel()

	qs := cqrs.NewQueries()
	cqrs.HandleQuery(qs, cqrs.HandlerFunc[getFailure, error](func(context.Context, getFailure) (error, error) {
		return nil, nil
	}))

	got, err := cqrs.SendQuery(context.Background(), qs, getFailure{})

	require.NoError(t, err)
	assert.NoError(t, got)
}

func TestStepsRunInTheOrderGivenAroundTheHandler(t *testing.T) {
	t.Parallel()

	var trail []string

	record := func(name string) cqrs.Step {
		return func(ctx context.Context, _ cqrs.Message, next cqrs.Next) (any, error) {
			trail = append(trail, name+" before")

			out, err := next(ctx)

			trail = append(trail, name+" after")

			return out, err
		}
	}

	cmds := cqrs.NewCommands(record("outer"), record("inner"))
	cqrs.HandleCommand(cmds, cqrs.HandlerFunc[CreateThing, written](
		func(context.Context, CreateThing) (written, error) {
			trail = append(trail, "handler")

			return written{}, nil
		},
	))

	_, err := cqrs.SendCommand(context.Background(), cmds, CreateThing{})

	require.NoError(t, err)
	assert.Equal(t, []string{"outer before", "inner before", "handler", "inner after", "outer after"}, trail)
}

func TestAStepSeesTheMessage(t *testing.T) {
	t.Parallel()

	var seen []cqrs.Message

	step := func(ctx context.Context, msg cqrs.Message, next cqrs.Next) (any, error) {
		seen = append(seen, msg)

		return next(ctx)
	}

	cmds := cqrs.NewCommands(step)
	cqrs.HandleCommand(cmds, cqrs.HandlerFunc[CreateThing, written](
		func(context.Context, CreateThing) (written, error) { return written{}, nil },
	))

	qs := cqrs.NewQueries(step)
	cqrs.HandleQuery(qs, cqrs.HandlerFunc[GetThing, thing](func(context.Context, GetThing) (thing, error) {
		return thing{}, nil
	}))

	_, err := cqrs.SendCommand(context.Background(), cmds, CreateThing{Title: "a"})
	require.NoError(t, err)

	_, err = cqrs.SendQuery(context.Background(), qs, GetThing{ID: "t1"})
	require.NoError(t, err)

	assert.Equal(t, []cqrs.Message{
		{Kind: cqrs.KindCommand, Name: "CreateThing", Body: CreateThing{Title: "a"}},
		{Kind: cqrs.KindQuery, Name: "GetThing", Body: GetThing{ID: "t1"}},
	}, seen)
}

func TestAStepCanStopTheMessage(t *testing.T) {
	t.Parallel()

	called := false
	deny := func(context.Context, cqrs.Message, cqrs.Next) (any, error) { return nil, errHandler }

	cmds := cqrs.NewCommands(deny)
	cqrs.HandleCommand(cmds, cqrs.HandlerFunc[CreateThing, written](
		func(context.Context, CreateThing) (written, error) {
			called = true

			return written{}, nil
		},
	))

	_, err := cqrs.SendCommand(context.Background(), cmds, CreateThing{})

	require.ErrorIs(t, err, errHandler)
	assert.False(t, called)
}
