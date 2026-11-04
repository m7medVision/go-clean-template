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

type createThing struct{ Title string }

func (createThing) CommandName() string { return "CreateThing" }

type getThing struct{ ID string }

func (getThing) QueryName() string { return "GetThing" }

type thing struct {
	ID    string
	Title string
}

func TestSendCommandReturnsTheHandlersResult(t *testing.T) {
	t.Parallel()

	cmds := cqrs.NewCommands()
	cqrs.HandleCommand(cmds, cqrs.HandlerFunc[createThing, cqrs.Written](
		func(_ context.Context, cmd createThing) (cqrs.Written, error) {
			return cqrs.Written{ID: "thing-" + cmd.Title, Position: 7}, nil
		},
	))

	written, err := cqrs.SendCommand[cqrs.Written](context.Background(), cmds, createThing{Title: "a"})

	require.NoError(t, err)
	assert.Equal(t, cqrs.Written{ID: "thing-a", Position: 7}, written)
}

func TestSendQueryReturnsTheHandlersResult(t *testing.T) {
	t.Parallel()

	qs := cqrs.NewQueries()
	cqrs.HandleQuery(qs, cqrs.HandlerFunc[getThing, thing](func(_ context.Context, q getThing) (thing, error) {
		return thing{ID: q.ID, Title: "found"}, nil
	}))

	got, err := cqrs.SendQuery[thing](context.Background(), qs, getThing{ID: "t1"})

	require.NoError(t, err)
	assert.Equal(t, thing{ID: "t1", Title: "found"}, got)
}

func TestHandlerErrorIsReturnedWithAZeroResult(t *testing.T) {
	t.Parallel()

	qs := cqrs.NewQueries()
	cqrs.HandleQuery(qs, cqrs.HandlerFunc[getThing, thing](func(context.Context, getThing) (thing, error) {
		return thing{ID: "partial"}, errHandler
	}))

	got, err := cqrs.SendQuery[thing](context.Background(), qs, getThing{})

	require.ErrorIs(t, err, errHandler)
	assert.Equal(t, thing{}, got)
}

func TestSendWithoutAHandlerIsErrNoHandler(t *testing.T) {
	t.Parallel()

	_, err := cqrs.SendCommand[cqrs.Written](context.Background(), cqrs.NewCommands(), createThing{})

	require.ErrorIs(t, err, cqrs.ErrNoHandler)
	assert.Contains(t, err.Error(), "CreateThing")
}

func TestASecondHandlerForOneTypePanics(t *testing.T) {
	t.Parallel()

	cmds := cqrs.NewCommands()
	h := cqrs.HandlerFunc[createThing, cqrs.Written](func(context.Context, createThing) (cqrs.Written, error) {
		return cqrs.Written{}, nil
	})
	cqrs.HandleCommand(cmds, h)

	assert.Panics(t, func() { cqrs.HandleCommand(cmds, h) })
}

func TestAResultOfAnotherTypeIsErrResultType(t *testing.T) {
	t.Parallel()

	qs := cqrs.NewQueries()
	cqrs.HandleQuery(qs, cqrs.HandlerFunc[getThing, thing](func(context.Context, getThing) (thing, error) {
		return thing{}, nil
	}))

	_, err := cqrs.SendQuery[string](context.Background(), qs, getThing{})

	require.ErrorIs(t, err, cqrs.ErrResultType)
}

func TestANilInterfaceResultIsTheZeroValue(t *testing.T) {
	t.Parallel()

	qs := cqrs.NewQueries()
	cqrs.HandleQuery(qs, cqrs.HandlerFunc[getThing, error](func(context.Context, getThing) (error, error) {
		return nil, nil
	}))

	got, err := cqrs.SendQuery[error](context.Background(), qs, getThing{})

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
	cqrs.HandleCommand(cmds, cqrs.HandlerFunc[createThing, cqrs.Written](
		func(context.Context, createThing) (cqrs.Written, error) {
			trail = append(trail, "handler")

			return cqrs.Written{}, nil
		},
	))

	_, err := cqrs.SendCommand[cqrs.Written](context.Background(), cmds, createThing{})

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
	cqrs.HandleCommand(cmds, cqrs.HandlerFunc[createThing, cqrs.Written](
		func(context.Context, createThing) (cqrs.Written, error) { return cqrs.Written{}, nil },
	))

	qs := cqrs.NewQueries(step)
	cqrs.HandleQuery(qs, cqrs.HandlerFunc[getThing, thing](func(context.Context, getThing) (thing, error) {
		return thing{}, nil
	}))

	_, err := cqrs.SendCommand[cqrs.Written](context.Background(), cmds, createThing{Title: "a"})
	require.NoError(t, err)

	_, err = cqrs.SendQuery[thing](context.Background(), qs, getThing{ID: "t1"})
	require.NoError(t, err)

	assert.Equal(t, []cqrs.Message{
		{Kind: cqrs.KindCommand, Name: "CreateThing", Body: createThing{Title: "a"}},
		{Kind: cqrs.KindQuery, Name: "GetThing", Body: getThing{ID: "t1"}},
	}, seen)
}

func TestAStepCanStopTheMessage(t *testing.T) {
	t.Parallel()

	called := false
	deny := func(context.Context, cqrs.Message, cqrs.Next) (any, error) { return nil, errHandler }

	cmds := cqrs.NewCommands(deny)
	cqrs.HandleCommand(cmds, cqrs.HandlerFunc[createThing, cqrs.Written](
		func(context.Context, createThing) (cqrs.Written, error) {
			called = true

			return cqrs.Written{}, nil
		},
	))

	_, err := cqrs.SendCommand[cqrs.Written](context.Background(), cmds, createThing{})

	require.ErrorIs(t, err, errHandler)
	assert.False(t, called)
}
