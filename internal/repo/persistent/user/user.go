// Package user implements the user read model on Postgres (the users and user_identities tables),
// the projector that writes it from the user events, and the importer that turns rows written
// before users stored events into events.
package user

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/m7medvision/go-clean-template/internal/entity"
	"github.com/m7medvision/go-clean-template/internal/repo"
	"github.com/m7medvision/go-clean-template/internal/repo/persistent/queries"
	"github.com/m7medvision/go-clean-template/pkg/postgres"
)

// Repo reads the users table.
type Repo struct {
	q *queries.Queries
}

// NewReadModel returns the user read model.
func NewReadModel(pg *postgres.Postgres) repo.UserReadModel {
	return &Repo{queries.New(pg.Pool)}
}

// ByID -.
func (r *Repo) ByID(ctx context.Context, id string) (entity.User, error) {
	user, err := r.q.GetUserByID(ctx, id)
	if err != nil {
		return entity.User{}, mapGetError(err, "ByID - r.q.GetUserByID")
	}

	return entity.User(user), nil
}

// ByEmail -.
func (r *Repo) ByEmail(ctx context.Context, email string) (entity.User, error) {
	user, err := r.q.GetUserByEmail(ctx, email)
	if err != nil {
		return entity.User{}, mapGetError(err, "ByEmail - r.q.GetUserByEmail")
	}

	return entity.User(user), nil
}

func mapGetError(err error, op string) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return entity.ErrUserNotFound
	}

	return fmt.Errorf("UserReadModel - %s: %w", op, err)
}
