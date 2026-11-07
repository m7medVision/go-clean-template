package user

import (
	"context"
	"fmt"

	"github.com/m7medvision/go-clean-template/internal/entity"
	"github.com/m7medvision/go-clean-template/internal/repo"
	"github.com/m7medvision/go-clean-template/internal/repo/persistent/queries"
	"github.com/m7medvision/go-clean-template/pkg/postgres"
)

// Projector keeps the user read model, the users and user_identities tables, up to date from the
// user events. The worker runs it; it writes through the transaction the runner's context carries.
type Projector struct {
	pg *postgres.Postgres
}

var _ repo.Subscriber = (*Projector)(nil)

// NewProjector returns the user projector writing to pg.
func NewProjector(pg *postgres.Postgres) *Projector {
	return &Projector{pg: pg}
}

// Name -.
func (*Projector) Name() string {
	return "user"
}

// Tables are what a rebuild empties.
func (*Projector) Tables() []string {
	return []string{"user_identities", "users"}
}

// Handles is every user event.
func (*Projector) Handles() []entity.Event {
	return entity.UserEvents()
}

// Handle -.
func (p *Projector) Handle(ctx context.Context, e *entity.RecordedEvent) error {
	return Project(ctx, queries.New(p.pg.Executor(ctx)), e)
}

// Project applies one event to the user tables through q.
func Project(ctx context.Context, q queries.Querier, e *entity.RecordedEvent) error {
	var err error

	switch ev := e.Event.(type) {
	case entity.UserRegistered:
		err = q.InsertUserRow(ctx, queries.InsertUserRowParams{
			ID: ev.UserID, Username: ev.Username, Email: ev.Email, PasswordHash: ev.PasswordHash,
			CreatedAt: ev.At, UpdatedAt: ev.At,
		})
	case entity.UserRegisteredViaOIDC:
		err = projectOIDCUser(ctx, q, &ev)
	case entity.UserImported:
		err = projectImport(ctx, q, &ev)
	default:
		err = entity.ErrUnhandledEvent
	}

	if err != nil {
		return fmt.Errorf("UserProjector - %s at %d: %w", e.Event.EventType(), e.Position, err)
	}

	return nil
}

func projectOIDCUser(ctx context.Context, q queries.Querier, ev *entity.UserRegisteredViaOIDC) error {
	err := q.InsertUserRow(ctx, queries.InsertUserRowParams{
		ID: ev.UserID, Username: ev.Username, Email: ev.Email, CreatedAt: ev.At, UpdatedAt: ev.At,
	})
	if err != nil {
		return err
	}

	return q.InsertIdentityRow(ctx, queries.InsertIdentityRowParams{
		Issuer: ev.Issuer, Subject: ev.Subject, UserID: ev.UserID, CreatedAt: ev.At,
	})
}

func projectImport(ctx context.Context, q queries.Querier, ev *entity.UserImported) error {
	err := q.InsertUserRow(ctx, queries.InsertUserRowParams{
		ID: ev.UserID, Username: ev.Username, Email: ev.Email, PasswordHash: ev.PasswordHash,
		CreatedAt: ev.CreatedAt, UpdatedAt: ev.UpdatedAt,
	})
	if err != nil {
		return err
	}

	for _, i := range ev.Identities {
		err = q.InsertIdentityRow(ctx, queries.InsertIdentityRowParams{
			Issuer: i.Issuer, Subject: i.Subject, UserID: ev.UserID, CreatedAt: i.CreatedAt,
		})
		if err != nil {
			return err
		}
	}

	return nil
}
