package user

import (
	"context"

	"github.com/m7medvision/go-clean-template/internal/entity"
	"github.com/m7medvision/go-clean-template/internal/repo/persistent/projection"
	"github.com/m7medvision/go-clean-template/internal/repo/persistent/queries"
	"github.com/m7medvision/go-clean-template/pkg/postgres"
)

// NewProjector keeps the user read model, the users and user_identities tables, up to date from the
// user events. The worker runs it.
func NewProjector(pg *postgres.Postgres) projection.Projector {
	return projection.NewProjector("user", []string{"user_identities", "users"}, entity.UserEvents(), pg, project)
}

// project applies one event to the user tables through q.
func project(ctx context.Context, q queries.Querier, e *entity.RecordedEvent) error {
	switch ev := e.Event.(type) {
	case entity.UserRegistered:
		return q.InsertUserRow(ctx, queries.InsertUserRowParams{
			ID: ev.UserID, Username: ev.Username, Email: ev.Email, PasswordHash: ev.PasswordHash,
			CreatedAt: ev.At, UpdatedAt: ev.At,
		})
	case entity.UserRegisteredViaOIDC:
		return projectOIDCUser(ctx, q, &ev)
	case entity.UserImported:
		return projectImport(ctx, q, &ev)
	default:
		return entity.ErrUnhandledEvent
	}
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
