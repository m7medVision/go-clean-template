// Package identity implements the Postgres-backed Identity repository.
package identity

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

const (
	_uniqueViolation = "23505"
	// _identityPKey is the primary key Atlas names for user_identities (issuer, subject).
	_identityPKey = "user_identities_pkey"
)

// Repo -.
type Repo struct {
	q *queries.Queries
}

// New returns an Identity repository instrumented with OpenTelemetry tracing spans.
func New(pg *postgres.Postgres) repo.IdentityRepo {
	return newTraced(&Repo{queries.New(pg.Pool)})
}

// UserByIdentity -.
func (r *Repo) UserByIdentity(ctx context.Context, issuer, subject string) (entity.User, error) {
	user, err := r.q.GetUserByIdentity(ctx, queries.GetUserByIdentityParams{Issuer: issuer, Subject: subject})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return entity.User{}, entity.ErrUserNotFound
		}

		return entity.User{}, fmt.Errorf("IdentityRepo - UserByIdentity - r.q.GetUserByIdentity: %w", err)
	}

	return entity.User(user), nil
}

// CreateWithIdentity -.
func (r *Repo) CreateWithIdentity(ctx context.Context, user *entity.User, issuer, subject string) error {
	err := r.q.CreateUserWithIdentity(ctx, queries.CreateUserWithIdentityParams{
		Issuer:    issuer,
		Subject:   subject,
		CreatedAt: user.CreatedAt,
		ID:        user.ID,
		Username:  user.Username,
		Email:     user.Email,
	})
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == _uniqueViolation {
			// Two first sign-ins of one identity raced, and the other one linked it.
			if pgErr.ConstraintName == _identityPKey {
				return entity.ErrUserAlreadyExists
			}

			return entity.ErrAccountConflict
		}

		return fmt.Errorf("IdentityRepo - CreateWithIdentity - r.q.CreateUserWithIdentity: %w", err)
	}

	return nil
}
