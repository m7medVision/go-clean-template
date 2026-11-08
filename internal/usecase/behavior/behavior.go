// Package behavior holds the application's own steps of the command and query pipelines: they
// know the domain (roles, metadata, the read model's consistency), which pkg/cqrs must not.
//
// The buses run them outermost first: Stamp, Limit, Wait, Retry, Authorize, Validate.
package behavior

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"time"

	"github.com/m7medvision/go-clean-template/internal/entity"
	"github.com/m7medvision/go-clean-template/internal/repo"
	"github.com/m7medvision/go-clean-template/pkg/cqrs"
)

const _limitTimeout = 500 * time.Millisecond

// Actor is a message that acts for a user, recorded on the events it causes.
type Actor interface {
	ActorID() string
}

// AdminOnly is a message only a caller holding entity.RoleAdmin may send. Every caller holds
// entity.RoleUser, so admin is the only role a message can require.
type AdminOnly interface {
	AdminOnly()
	Caller() entity.Principal
}

// Validator is a message that checks itself before its handler runs. Rules every transport shares
// go here; domain rules stay on the aggregate.
type Validator interface {
	Validate() error
}

type Limited interface {
	LimitScope() string
	LimitAccount() string
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

func Limit(limiter repo.Limiter, l *slog.Logger) cqrs.Step {
	return func(ctx context.Context, msg cqrs.Message, next cqrs.Next) (any, error) {
		m, ok := msg.Body.(Limited)
		if !ok {
			return next(ctx)
		}

		a := &attempt{limiter: limiter, l: l, scope: m.LimitScope(), account: m.LimitAccount()}

		if err := a.admit(ctx); err != nil {
			return nil, fmt.Errorf("behavior - Limit %s: %w", msg.Name, err)
		}

		out, err := next(ctx)
		a.settle(ctx, err)

		return out, err
	}
}

type attempt struct {
	limiter repo.Limiter
	l       *slog.Logger
	scope   string
	account string
	failed  bool
}

func (a *attempt) admit(ctx context.Context) error {
	if err := a.takeClient(ctx); err != nil {
		return err
	}

	return a.checkAccount(ctx)
}

func (a *attempt) takeClient(ctx context.Context) error {
	client, ok := entity.ClientFromContext(ctx)
	if !ok {
		return nil
	}

	limitCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), _limitTimeout)
	defer cancel()

	allowance, err := a.limiter.TakeClient(limitCtx, a.scope, client)

	return a.refusal(ctx, allowance, err)
}

func (a *attempt) checkAccount(ctx context.Context) error {
	if a.account == "" || a.failed {
		return nil
	}

	limitCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), _limitTimeout)
	defer cancel()

	// Checking and counting are separate calls, so concurrent wrong passwords can pass a few over
	// the budget before it locks.
	allowance, err := a.limiter.CheckAccount(limitCtx, a.scope, a.account)

	return a.refusal(ctx, allowance, err)
}

func (a *attempt) refusal(ctx context.Context, allowance entity.Allowance, err error) error {
	switch {
	case err != nil:
		a.failOpen(ctx, err)

		return nil
	case !allowance.Allowed:
		return entity.TooManyAttempts(allowance.RetryAfter)
	default:
		return nil
	}
}

func (a *attempt) settle(ctx context.Context, err error) {
	if a.account == "" || a.failed {
		return
	}

	switch {
	case err == nil:
		a.record(ctx, a.limiter.ClearAccount)
	case errors.Is(err, entity.ErrInvalidCredentials):
		a.record(ctx, a.limiter.TakeAccountFailure)
	}
}

func (a *attempt) record(ctx context.Context, update func(ctx context.Context, scope, account string) error) {
	limitCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), _limitTimeout)
	defer cancel()

	if err := update(limitCtx, a.scope, a.account); err != nil {
		a.failOpen(ctx, err)
	}
}

func (a *attempt) failOpen(ctx context.Context, err error) {
	a.failed = true
	// An outage of the counter store must not stop every sign-in, so its error lets the attempt
	// through.
	a.l.WarnContext(ctx, "sign-in limit unavailable", "scope", a.scope, "error", err)
}

// Wait holds a command's result until the projectors of the events it wrote (an entity.Write, alone
// or embedded) have handled the event at its position, checking every tick, so a read sent straight
// after the command sees the write. projectors names, for each event type, the projectors that
// handle it. A projector that handles none of the events written is not waited for, so one that is
// stuck holds up only the writes it projects; a command that wrote no events waits for nothing.
// Past timeout it returns entity.ErrReadModelBehind: the write itself is committed.
func Wait(checkpoints repo.ProjectionCheckpoints, projectors map[string][]string, timeout, tick time.Duration) cqrs.Step {
	return func(ctx context.Context, msg cqrs.Message, next cqrs.Next) (any, error) {
		out, err := next(ctx)
		if err != nil {
			return out, err
		}

		written, ok := out.(entity.Write)
		if !ok {
			return out, nil
		}

		names := projecting(projectors, written.WrittenEvents())
		if len(names) == 0 {
			return out, nil
		}

		if err = waitFor(ctx, checkpoints, names, written.WrittenAt(), timeout, tick); err != nil {
			return out, fmt.Errorf("behavior - Wait %s: %w", msg.Name, err)
		}

		return out, nil
	}
}

// projecting lists, once each, the projectors of the event types written.
func projecting(projectors map[string][]string, written []string) []string {
	var names []string

	for _, eventType := range written {
		for _, name := range projectors[eventType] {
			if !slices.Contains(names, name) {
				names = append(names, name)
			}
		}
	}

	return names
}

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

// Authorize stops an AdminOnly message whose caller is not an admin with entity.ErrAdminRequired.
func Authorize() cqrs.Step {
	return func(ctx context.Context, msg cqrs.Message, next cqrs.Next) (any, error) {
		if a, ok := msg.Body.(AdminOnly); ok && !a.Caller().Has(entity.RoleAdmin) {
			return nil, entity.ErrAdminRequired
		}

		return next(ctx)
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
