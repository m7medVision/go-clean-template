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
	// ErrResultType is returned when a handler's result is not the type the sender asked for.
	ErrResultType = errors.New("cqrs: result type")
)

// Command is a request to change state. CommandName labels its traces, logs and metrics.
type Command interface {
	CommandName() string
}

// Query is a request to read state. QueryName labels its traces, logs and metrics.
type Query interface {
	QueryName() string
}

// Handler handles one message type.
type Handler[M, R any] interface {
	Handle(ctx context.Context, msg M) (R, error)
}

// HandlerFunc adapts a function to a Handler.
type HandlerFunc[M, R any] func(ctx context.Context, msg M) (R, error)

// Handle calls f.
func (f HandlerFunc[M, R]) Handle(ctx context.Context, msg M) (R, error) {
	return f(ctx, msg)
}

// Written is what a command handler returns: the aggregate it wrote and the store position of the
// last event it appended. A command never returns resource data; read that with a query.
type Written struct {
	ID       string
	Position int64
}

// WrittenAt is the position of the write. A result that embeds Written, such as a sign-in that also
// carries a token, has it too, so the pipeline can tell what it wrote.
func (w Written) WrittenAt() int64 {
	return w.Position
}

// Kind names the bus a message travels on.
type Kind string

const (
	// KindCommand is the command bus.
	KindCommand Kind = "command"
	// KindQuery is the query bus.
	KindQuery Kind = "query"
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
