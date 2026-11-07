package cqrs

import (
	"context"
	"fmt"
	"reflect"
	"slices"
)

// Commands is the command bus.
type Commands struct {
	bus bus
}

// Queries is the query bus.
type Queries struct {
	bus bus
}

// NewCommands returns a command bus whose messages pass through steps, the first outermost.
func NewCommands(steps ...Step) *Commands {
	return &Commands{bus: newBus(KindCommand, steps)}
}

// NewQueries returns a query bus whose messages pass through steps, the first outermost.
func NewQueries(steps ...Step) *Queries {
	return &Queries{bus: newBus(KindQuery, steps)}
}

// HandleCommand registers h as the handler of C. A second handler for C panics: wiring happens
// once, at startup, and a duplicate is a programming error.
func HandleCommand[C CommandOf[R], R any](b *Commands, h Handler[C, R]) {
	register(&b.bus, h)
}

// HandleQuery registers h as the handler of Q. A second handler for Q panics.
func HandleQuery[Q QueryOf[R], R any](b *Queries, h Handler[Q, R]) {
	register(&b.bus, h)
}

// SendCommand runs cmd through the command bus's steps to its handler and returns the handler's
// result. A type with no handler is ErrNoHandler.
func SendCommand[C CommandOf[R], R any](ctx context.Context, b *Commands, cmd C) (R, error) {
	return send[R](ctx, &b.bus, cmd)
}

// SendQuery runs q through the query bus's steps to its handler and returns the handler's result.
// A type with no handler is ErrNoHandler.
func SendQuery[Q QueryOf[R], R any](ctx context.Context, b *Queries, q Q) (R, error) {
	return send[R](ctx, &b.bus, q)
}

// handle is a registered handler with its message type erased.
type handle func(ctx context.Context, msg any) (any, error)

type bus struct {
	kind     Kind
	steps    []Step
	handlers map[reflect.Type]handle
}

func newBus(kind Kind, steps []Step) bus {
	return bus{kind: kind, steps: steps, handlers: map[reflect.Type]handle{}}
}

func register[M, R any](b *bus, h Handler[M, R]) {
	t := reflect.TypeFor[M]()
	if _, ok := b.handlers[t]; ok {
		panic(fmt.Sprintf("cqrs: a second %s handler for %s", b.kind, t))
	}

	b.handlers[t] = func(ctx context.Context, msg any) (any, error) {
		return h.Handle(ctx, msg.(M)) //nolint:errcheck // send looks h up by the type of msg, so msg is an M
	}
}

func send[R, M any](ctx context.Context, b *bus, msg M) (R, error) {
	var zero R

	name := reflect.TypeFor[M]().Name()

	h, ok := b.handlers[reflect.TypeFor[M]()]
	if !ok {
		return zero, fmt.Errorf("%w: %s %s", ErrNoHandler, b.kind, name)
	}

	m := Message{Kind: b.kind, Name: name, Body: msg}
	run := Next(func(ctx context.Context) (any, error) { return h(ctx, msg) })

	for _, v := range slices.Backward(b.steps) {
		step, next := v, run
		run = func(ctx context.Context) (any, error) { return step(ctx, m, next) }
	}

	out, err := run(ctx)
	if err != nil {
		return zero, err
	}

	if out == nil {
		return zero, nil
	}

	r, ok := out.(R)
	if !ok {
		return zero, fmt.Errorf("%w: %s %s returned %T, not %s", ErrResultType, b.kind, name, out, reflect.TypeFor[R]())
	}

	return r, nil
}
