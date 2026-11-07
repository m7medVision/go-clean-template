package entity

import (
	"context"
	"slices"
	"time"
)

// Event is a fact an aggregate recorded. Events are the stored truth: an aggregate's state is
// whatever its events add up to.
//
// EventType names the event in the store and never changes once events of the type are stored.
// SchemaVersion is the version of the event's current shape; changing the shape means a new
// version and an upcaster in the store's codec that rewrites the previous one.
type Event interface {
	EventType() string
	SchemaVersion() int
}

// EventTypes lists one zero value of every Event type. The store decodes only these, so a new
// event needs an entry here; each domain's events live in its <name>_events.go file.
func EventTypes() []Event {
	return slices.Concat(
		taskEvents(),
		userEvents(),
		conversionEvents(),
		passkeyEvents(),
	)
}

// Metadata travels with every stored event: who caused it, the trace it belongs to
// (correlation), and the message that caused it (causation).
type Metadata struct {
	UserID        string `json:"user_id,omitempty"`
	CorrelationID string `json:"correlation_id,omitempty"`
	CausationID   string `json:"causation_id,omitempty"`
}

// RecordedEvent is an event as the store holds it.
type RecordedEvent struct {
	// Position orders every event in the store, across streams.
	Position int64
	StreamID string
	// StreamVersion is the event's place in its stream, from 1.
	StreamVersion int
	Event         Event
	Metadata      Metadata
	RecordedAt    time.Time
}

type metadataKey struct{}

// ContextWithMetadata returns ctx carrying md, for the store to stamp on the events it appends.
func ContextWithMetadata(ctx context.Context, md Metadata) context.Context {
	return context.WithValue(ctx, metadataKey{}, md)
}

// MetadataFromContext returns the metadata ctx carries, or none.
func MetadataFromContext(ctx context.Context) Metadata {
	md, _ := ctx.Value(metadataKey{}).(Metadata) //nolint:errcheck // a context without metadata has none

	return md
}

// Written is what a command that writes answers: the aggregate it wrote and the store position of
// the last event it appended. A command never returns resource data; read that with a query. A zero
// Position means the command appended nothing, as when a sign-in finds the account it signs in to.
type Written struct {
	ID       string
	Position int64
}

// Write is a command result that carries a Written: a Written itself, or a result that embeds one,
// such as a sign-in that also carries a token. The command bus's wait step waits for its position.
type Write interface {
	WrittenAt() int64
}

var _ Write = Written{}

// SignedIn is what every sign-in answers: the user it signed in (with the position of their
// creation when the sign-in created them, else 0) and the auth token.
type SignedIn struct {
	Written

	Token string
}

// WrittenAt is the position of the write.
func (w Written) WrittenAt() int64 {
	return w.Position
}
