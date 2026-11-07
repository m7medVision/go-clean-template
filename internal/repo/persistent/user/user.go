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

// Repo reads the users table. Its queries join the transaction the context carries.
type Repo struct {
	pg *postgres.Postgres
}

// NewReadModel returns the user read model.
func NewReadModel(pg *postgres.Postgres) repo.UserReadModel {
	return &Repo{pg: pg}
}

func (r *Repo) q(ctx context.Context) *queries.Queries {
	return queries.New(r.pg.Executor(ctx))
}

// ByID -.
func (r *Repo) ByID(ctx context.Context, id string) (entity.User, error) {
	user, err := r.q(ctx).GetUserByID(ctx, id)
	if err != nil {
		return entity.User{}, mapGetError(err, "ByID - r.q(ctx).GetUserByID")
	}

	return entity.User(user), nil
}

// ByEmail -.
func (r *Repo) ByEmail(ctx context.Context, email string) (entity.User, error) {
	user, err := r.q(ctx).GetUserByEmail(ctx, email)
	if err != nil {
		return entity.User{}, mapGetError(err, "ByEmail - r.q(ctx).GetUserByEmail")
	}

	return entity.User(user), nil
}

func mapGetError(err error, op string) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return entity.ErrUserNotFound
	}

	return fmt.Errorf("UserReadModel - %s: %w", op, err)
}
