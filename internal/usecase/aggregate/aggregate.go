// Package aggregate is what every command handler does with the event store: load an aggregate, let
// the command change it, and save the events it recorded.
package aggregate

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/m7medvision/go-clean-template/internal/entity"
	"github.com/m7medvision/go-clean-template/internal/repo"
)

// Identified is an aggregate that knows its own ID, which a write answers with.
type Identified interface {
	entity.Aggregate
	ID() string
}

// Loadable is a pointer to an aggregate type that can be found by ID: StreamOf names an ID's
// stream, or answers NotFound for an ID that names none, and NotFound is the domain's error for an
// ID with no stream.
type Loadable[T any] interface {
	*T
	Identified
	StreamOf(id string) (string, error)
	NotFound() error
}

// Load loads the aggregate with id. An ID with no stream, or one that names no stream, is the
// aggregate's NotFound error.
func Load[T any, A Loadable[T]](ctx context.Context, store repo.EventStore, id string) (A, error) {
	a := A(new(T))

	stream, err := a.StreamOf(id)
	if err != nil {
		return nil, err
	}

	if err = store.Load(ctx, stream, a); err != nil {
		if errors.Is(err, entity.ErrStreamNotFound) {
			return nil, a.NotFound()
		}

		return nil, fmt.Errorf("store.Load: %w", err)
	}

	return a, nil
}

// Save saves the events a recorded, new or loaded, with claims.
func Save(ctx context.Context, store repo.EventStore, a Identified, claims ...entity.Claim) (entity.Written, error) {
	types := make([]string, len(a.Pending()))
	for i, e := range a.Pending() {
		types[i] = e.EventType()
	}

	position, err := store.Save(ctx, a, claims...)
	if err != nil {
		return entity.Written{}, fmt.Errorf("store.Save: %w", err)
	}

	return entity.Written{ID: a.ID(), Position: position, Events: types}, nil
}

// Change loads the aggregate with id, as Load does, runs change on it, and saves the events it
// recorded with claims. An error from change is returned as it is, and nothing is saved.
func Change[T any, A Loadable[T]](
	ctx context.Context, store repo.EventStore, id string, change func(A) error, claims ...entity.Claim,
) (entity.Written, error) {
	a, err := Load[T, A](ctx, store, id)
	if err != nil {
		return entity.Written{}, err
	}

	if err := change(a); err != nil {
		return entity.Written{}, err
	}

	return Save(ctx, store, a, claims...)
}

// Now is clock's time (time.Now when clock is nil) in UTC.
func Now(clock func() time.Time) time.Time {
	if clock == nil {
		clock = time.Now
	}

	return clock().UTC()
}
