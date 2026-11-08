// Package repo holds the ports the use cases call: the ones every domain shares here, and each
// domain's in the file named after it (task.go, user.go, …). go:generate writes gomock mocks of the
// ones a test constructs to repomock/, one file per contract file (make mock). The packages under
// internal/repo implement them.
package repo

import (
	"context"
	"net/netip"

	"github.com/m7medvision/go-clean-template/internal/entity"
)

//go:generate mockgen -source=contracts.go -destination=repomock/contracts.go -package=repomock -exclude_interfaces=EventStore,Subscriber,ProjectionCheckpoints,SessionIssuer

type (
	// EventStore keeps aggregates as streams of events.
	//
	// Save appends a's pending events, and reserves or releases claims, in one transaction (it joins
	// the one the context carries), then marks a saved and returns the position of its last event.
	// A stream that moved on since a was loaded is entity.ErrConcurrencyConflict; a claim another
	// owner holds is that claim's Taken error. Saving with nothing pending writes nothing and
	// returns 0.
	//
	// Load replays the stream into a fresh a: its latest snapshot, then the events after it. A
	// stream with no events is entity.ErrStreamNotFound. ClaimOwner returns who holds a claim, or
	// entity.ErrClaimNotFound.
	EventStore interface {
		Save(ctx context.Context, a entity.Aggregate, claims ...entity.Claim) (int64, error)
		Load(ctx context.Context, streamID string, a entity.Aggregate) error
		ClaimOwner(ctx context.Context, scope, value string) (string, error)
	}

	// Subscriber is a projector or an event handler. The projection runner hands it, in order, every
	// stored event of a type it Handles (one zero value per type), inside the transaction that moves
	// its checkpoint, so whatever it writes through the context's transaction commits exactly once
	// with the checkpoint. Events of other types move its checkpoint without reaching it.
	Subscriber interface {
		Name() string
		Handles() []entity.Event
		Handle(ctx context.Context, e *entity.RecordedEvent) error
	}

	// ProjectionCheckpoints tells how far the projectors have read the event store. Reached reports
	// whether every projector in names has handled the event at position.
	ProjectionCheckpoints interface {
		Reached(ctx context.Context, names []string, position int64) (bool, error)
	}

	// SessionIssuer signs the auth token a sign-in answers with, carrying the user's roles.
	SessionIssuer interface {
		Issue(userID string, roles []entity.Role) (string, error)
	}

	Limiter interface {
		TakeClient(ctx context.Context, scope string, client netip.Addr) (entity.Allowance, error)
		CheckAccount(ctx context.Context, scope, account string) (entity.Allowance, error)
		TakeAccountFailure(ctx context.Context, scope, account string) error
		ClearAccount(ctx context.Context, scope, account string) error
	}
)
