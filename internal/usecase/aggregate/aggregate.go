// Package aggregate is what every command handler does with the event store: create a new
// aggregate, or load one, let the command change it, and save the events it recorded.
package aggregate

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/m7medvision/go-clean-template/internal/entity"
	"github.com/m7medvision/go-clean-template/internal/repo"
	"github.com/m7medvision/go-clean-template/pkg/cqrs"
)

// Create saves a, a new aggregate with the given id, with claims.
func Create(ctx context.Context, store repo.EventStore, id string, a entity.Aggregate, claims ...entity.Claim) (
	cqrs.Written, error,
) {
	position, err := store.Save(ctx, a, claims...)
	if err != nil {
		return cqrs.Written{}, fmt.Errorf("store.Save: %w", err)
	}

	return cqrs.Written{ID: id, Position: position}, nil
}

// Change loads streamID into a, the aggregate with the given id, runs change on it, and saves the
// events it recorded with claims. A stream with no events is notFound, the domain's own error;
// an error from change is returned as it is, and nothing is saved.
func Change[A entity.Aggregate](
	ctx context.Context, store repo.EventStore, id, streamID string, a A, notFound error, change func(A) error,
	claims ...entity.Claim,
) (cqrs.Written, error) {
	if err := store.Load(ctx, streamID, a); err != nil {
		if errors.Is(err, entity.ErrStreamNotFound) {
			return cqrs.Written{}, notFound
		}

		return cqrs.Written{}, fmt.Errorf("store.Load: %w", err)
	}

	if err := change(a); err != nil {
		return cqrs.Written{}, err
	}

	return Create(ctx, store, id, a, claims...)
}

// Now is clock's time (time.Now when clock is nil) in UTC, to the microsecond: the precision the
// read model's timestamp columns keep, so an event and the row projected from it agree exactly.
func Now(clock func() time.Time) time.Time {
	if clock == nil {
		clock = time.Now
	}

	return clock().UTC().Truncate(time.Microsecond)
}
