package entity

import "encoding/json"

// Aggregate is a write-side object rebuilt from its own stream of events. Its behavior methods
// check the domain's rules and record new events; Apply changes its state for one event, whether
// it was just recorded or replayed from the store. Every aggregate embeds AggregateBase.
type Aggregate interface {
	// StreamID names the aggregate's stream, such as "task-<id>".
	StreamID() string
	Apply(e Event)
	Version() int
	Pending() []Event

	base() *AggregateBase
}

// Snapshotter is an aggregate the store can save whole, so loading a long stream replays only the
// events after the snapshot. SnapshotVersion is the version of the snapshot's shape: a stored
// snapshot of another version is ignored and the stream is replayed from its first event.
type Snapshotter interface {
	Aggregate
	SnapshotVersion() int
	// Snapshot returns the state to store, encoded as JSON.
	Snapshot() any
	RestoreSnapshot(data json.RawMessage) error
}

// AggregateBase is what every aggregate shares: the version it was loaded at and the events it
// has recorded since.
type AggregateBase struct {
	version int
	pending []Event
}

// Version is the stream version the aggregate was loaded at: the version the store expects to
// find when it appends the pending events. A new aggregate is at 0.
func (a *AggregateBase) Version() int {
	return a.version
}

// Pending returns the events recorded since the aggregate was loaded or last saved.
func (a *AggregateBase) Pending() []Event {
	return a.pending
}

func (a *AggregateBase) base() *AggregateBase {
	return a
}

// Replay applies events the store read, without queueing them, and leaves a at version: the
// stream version of the last of them.
func Replay(a Aggregate, version int, events ...Event) {
	for _, e := range events {
		a.Apply(e)
	}

	a.base().version = version
}

// Saved tells a that the store appended its pending events: its version moves past them and
// nothing is pending any more.
func Saved(a Aggregate) {
	b := a.base()
	b.version += len(b.pending)
	b.pending = nil
}

// Record applies e to a and queues it for the store. Aggregates call it from their behavior
// methods, once their rules have passed; nothing else should, or a rule is skipped.
func Record(a Aggregate, e Event) {
	a.Apply(e)

	b := a.base()
	b.pending = append(b.pending, e)
}

// Claim reserves Value in Scope for Owner, unique across every aggregate: a user's email, an SSO
// identity. A command hands its claims to the store with its events, and both commit or neither
// does. Release gives a value back. Taken is the error the store returns when another owner holds
// the value, so each domain keeps its own error, such as ErrUserAlreadyExists.
type Claim struct {
	Scope   string
	Value   string
	Owner   string
	Release bool
	Taken   error
}
