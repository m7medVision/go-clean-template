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
// stream, and NotFound is the domain's error for an ID with none.
type Loadable[T any] interface {
	*T
	Identified
	StreamOf(id string) string
	NotFound() error
}

// Save saves the events a recorded, new or loaded, with claims.
func Save(ctx context.Context, store repo.EventStore, a Identified, claims ...entity.Claim) (entity.Written, error) {
	position, err := store.Save(ctx, a, claims...)
	if err != nil {
		return entity.Written{}, fmt.Errorf("store.Save: %w", err)
	}

	return entity.Written{ID: a.ID(), Position: position}, nil
}

// Change loads the aggregate with id, runs change on it, and saves the events it recorded with
// claims. An ID with no stream is the aggregate's NotFound error; an error from change is returned
// as it is, and nothing is saved.
func Change[T any, A Loadable[T]](
	ctx context.Context, store repo.EventStore, id string, change func(A) error, claims ...entity.Claim,
) (entity.Written, error) {
	a := A(new(T))

	if err := store.Load(ctx, a.StreamOf(id), a); err != nil {
		if errors.Is(err, entity.ErrStreamNotFound) {
			return entity.Written{}, a.NotFound()
		}

		return entity.Written{}, fmt.Errorf("store.Load: %w", err)
	}

	if err := change(a); err != nil {
		return entity.Written{}, err
	}

	return Save(ctx, store, a, claims...)
}

// Now is clock's time (time.Now when clock is nil) in UTC, to the microsecond: the precision the
// read model's timestamp columns keep, so an event and the row projected from it agree exactly.
func Now(clock func() time.Time) time.Time {
	if clock == nil {
		clock = time.Now
	}

	return clock().UTC().Truncate(time.Microsecond)
}
