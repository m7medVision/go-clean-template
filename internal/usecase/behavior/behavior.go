// Package behavior holds the application's own steps of the command and query pipelines: they
// know the domain (roles, metadata, the read model's consistency), which pkg/cqrs must not.
//
// The buses run them in this order, outermost first: Stamp, Wait, Retry, Authorize, Validate.
package behavior

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/m7medvision/go-clean-template/internal/entity"
	"github.com/m7medvision/go-clean-template/internal/repo"
	"github.com/m7medvision/go-clean-template/pkg/cqrs"
)

// Actor is a message that acts for a user, recorded on the events it causes.
type Actor interface {
	ActorID() string
}

// Restricted is a message only a caller holding a role may send.
type Restricted interface {
	RequiredRole() entity.Role
	Caller() entity.Principal
}

// Validator is a message that checks itself before its handler runs. Rules every transport shares
// go here; domain rules stay on the aggregate.
type Validator interface {
	Validate() error
}

// Stamp records who sent a message and which message it was in the context's event metadata, so
// the events its handler appends carry them.
func Stamp() cqrs.Step {
	return func(ctx context.Context, msg cqrs.Message, next cqrs.Next) (any, error) {
		md := entity.MetadataFromContext(ctx)
		md.CausationID = string(msg.Kind) + " " + msg.Name

		if a, ok := msg.Body.(Actor); ok {
			md.UserID = a.ActorID()
		}

		return next(entity.ContextWithMetadata(ctx, md))
	}
}

// Wait holds a command's result until every projector in projectors has handled the event at its
// position (cqrs.Written, alone or embedded), checking every tick, so a read sent straight after the
// command sees the write.
// Past timeout it returns entity.ErrReadModelBehind: the write itself is committed.
func Wait(checkpoints repo.ProjectionCheckpoints, projectors []string, timeout, tick time.Duration) cqrs.Step {
	return func(ctx context.Context, msg cqrs.Message, next cqrs.Next) (any, error) {
		out, err := next(ctx)
		if err != nil || len(projectors) == 0 {
			return out, err
		}

		written, ok := out.(interface{ WrittenAt() int64 })
		if !ok || written.WrittenAt() == 0 {
			return out, nil
		}

		if err = waitFor(ctx, checkpoints, projectors, written.WrittenAt(), timeout, tick); err != nil {
			return out, fmt.Errorf("behavior - Wait %s: %w", msg.Name, err)
		}

		return out, nil
	}
}

// waitFor polls the projectors' checkpoints every tick until all reach position, or timeout.
func waitFor(
	ctx context.Context, checkpoints repo.ProjectionCheckpoints, projectors []string, position int64,
	timeout, tick time.Duration,
) error {
	deadline := time.NewTimer(timeout)
	defer deadline.Stop()

	for {
		reached, err := checkpoints.Reached(ctx, projectors, position)
		if err != nil || reached {
			return err
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-deadline.C:
			return fmt.Errorf("position %d: %w", position, entity.ErrReadModelBehind)
		case <-time.After(tick):
		}
	}
}

// Retry runs a command's handler again, up to attempts times in all, while it fails with
// entity.ErrConcurrencyConflict. A handler loads its aggregate afresh each time, so the next attempt
// starts from the write that got there first.
func Retry(attempts int) cqrs.Step {
	return func(ctx context.Context, _ cqrs.Message, next cqrs.Next) (any, error) {
		var (
			out any
			err error
		)

		for range max(attempts, 1) {
			out, err = next(ctx)
			if !errors.Is(err, entity.ErrConcurrencyConflict) {
				return out, err
			}
		}

		return out, err
	}
}

// Authorize stops a Restricted message whose caller lacks its role: entity.ErrAdminRequired for
// the admin role, entity.ErrRoleRequired for any other.
func Authorize() cqrs.Step {
	return func(ctx context.Context, msg cqrs.Message, next cqrs.Next) (any, error) {
		r, ok := msg.Body.(Restricted)
		if !ok || r.Caller().Has(r.RequiredRole()) {
			return next(ctx)
		}

		if r.RequiredRole() == entity.RoleAdmin {
			return nil, entity.ErrAdminRequired
		}

		return nil, fmt.Errorf("%w: %s", entity.ErrRoleRequired, r.RequiredRole())
	}
}

// Validate stops a message whose Validate fails, before its handler runs.
func Validate() cqrs.Step {
	return func(ctx context.Context, msg cqrs.Message, next cqrs.Next) (any, error) {
		if v, ok := msg.Body.(Validator); ok {
			if err := v.Validate(); err != nil {
				return nil, fmt.Errorf("behavior - Validate %s: %w", msg.Name, err)
			}
		}

		return next(ctx)
	}
}
