// Package user implements the Postgres-backed User repository.
package user

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/m7medvision/go-clean-template/internal/entity"
	"github.com/m7medvision/go-clean-template/internal/repo"
	"github.com/m7medvision/go-clean-template/internal/repo/persistent/queries"
	"github.com/m7medvision/go-clean-template/pkg/postgres"
)

// Repo -.
type Repo struct {
	q *queries.Queries
}

// New returns a User repository instrumented with OpenTelemetry tracing spans.
func New(pg *postgres.Postgres) repo.UserRepo {
	return newTraced(&Repo{queries.New(pg.Pool)})
}

// Store -.
func (r *Repo) Store(ctx context.Context, user *entity.User) error {
	err := r.q.CreateUser(ctx, queries.CreateUserParams(*user))
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return entity.ErrUserAlreadyExists
		}

		return fmt.Errorf("UserRepo - Store - r.q.CreateUser: %w", err)
	}

	return nil
}

// GetByID -.
func (r *Repo) GetByID(ctx context.Context, id string) (entity.User, error) {
	user, err := r.q.GetUserByID(ctx, id)
	if err != nil {
		return entity.User{}, mapGetError(err, "GetByID - r.q.GetUserByID")
	}

	return entity.User(user), nil
}

// GetByEmail -.
func (r *Repo) GetByEmail(ctx context.Context, email string) (entity.User, error) {
	user, err := r.q.GetUserByEmail(ctx, email)
	if err != nil {
		return entity.User{}, mapGetError(err, "GetByEmail - r.q.GetUserByEmail")
	}

	return entity.User(user), nil
}

func mapGetError(err error, op string) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return entity.ErrUserNotFound
	}

	return fmt.Errorf("UserRepo - %s: %w", op, err)
}
