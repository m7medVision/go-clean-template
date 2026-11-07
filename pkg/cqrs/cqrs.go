// Package cqrs carries commands and queries to their handlers through a pipeline of steps.
//
// A command asks to change state and a query asks to read it; they travel on separate buses, so a
// query can never be sent where a command belongs. Every message type has exactly one handler,
// registered at startup. Steps wrap every message on a bus, outermost first: tracing, logging and
// metrics live here, and the application adds its own.
package cqrs

import (
	"context"
	"errors"
)

var (
	// ErrNoHandler is returned for a message type nothing has registered a handler for.
	ErrNoHandler = errors.New("cqrs: no handler")
	// ErrResultType is returned when a step answers with another type than the message's result.
	ErrResultType = errors.New("cqrs: result type")
)

// Command marks a request to change state that answers R. A command type embeds it, and the bus
// names the message after the type:
//
//	type CreateTask struct {
//		cqrs.Command[entity.Written]
//
//		Title string
//	}
type Command[R any] struct{}

func (Command[R]) command(R) {}

// Query marks a request to read state that answers R, as Command does for a command.
type Query[R any] struct{}

func (Query[R]) query(R) {}

// CommandOf is a message that embeds Command[R]: a command that answers R.
type CommandOf[R any] interface {
	command(R)
}

// QueryOf is a message that embeds Query[R]: a query that answers R.
type QueryOf[R any] interface {
	query(R)
}

type Handler[M, R any] interface {
	Handle(ctx context.Context, msg M) (R, error)
}

// HandlerFunc adapts a function to a Handler.
type HandlerFunc[M, R any] func(ctx context.Context, msg M) (R, error)

func (f HandlerFunc[M, R]) Handle(ctx context.Context, msg M) (R, error) {
	return f(ctx, msg)
}

// Kind names the bus a message travels on.
type Kind string

const (
	KindCommand Kind = "command"
	KindQuery   Kind = "query"
)

// Message is what a step sees of the message it wraps. Body is the command or query itself, so a
// step can check it for an optional method such as Validate.
type Message struct {
	Kind Kind
	Name string
	Body any
}

// Next runs the rest of the pipeline and then the handler.
type Next func(ctx context.Context) (any, error)

// Step wraps every message on a bus. It calls next to go on, or returns without calling it to stop
// the message.
type Step func(ctx context.Context, msg Message, next Next) (any, error)

// Buses are the command bus and the query bus together, as the transports take them.
type Buses struct {
	Commands *Commands
	Queries  *Queries
}
