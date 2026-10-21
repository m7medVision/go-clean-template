// Package passkey implements the Postgres-backed Passkey repository.
package passkey

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/m7medvision/go-clean-template/internal/entity"
	"github.com/m7medvision/go-clean-template/internal/repo"
	"github.com/m7medvision/go-clean-template/internal/repo/persistent/queries"
	"github.com/m7medvision/go-clean-template/pkg/postgres"
)

const _uniqueViolation = "23505"

// Repo -.
type Repo struct {
	q *queries.Queries
}

// New returns a Passkey repository instrumented with OpenTelemetry tracing spans.
func New(pg *postgres.Postgres) repo.PasskeyRepo {
	return newTraced(&Repo{queries.New(pg.Pool)})
}

// Store -.
func (r *Repo) Store(ctx context.Context, passkey *entity.Passkey) error {
	err := r.q.StorePasskey(ctx, queries.StorePasskeyParams(*passkey))
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == _uniqueViolation {
			return entity.ErrPasskeyExists
		}

		return fmt.Errorf("PasskeyRepo - Store - r.q.StorePasskey: %w", err)
	}

	return nil
}

// ListByUser returns the user's passkeys, oldest first.
func (r *Repo) ListByUser(ctx context.Context, userID string) ([]entity.Passkey, error) {
	rows, err := r.q.ListPasskeysByUser(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("PasskeyRepo - ListByUser - r.q.ListPasskeysByUser: %w", err)
	}

	passkeys := make([]entity.Passkey, len(rows))
	for i := range rows {
		passkeys[i] = entity.Passkey(rows[i])
	}

	return passkeys, nil
}

// GetByID -.
func (r *Repo) GetByID(ctx context.Context, id entity.PasskeyID) (entity.Passkey, error) {
	passkey, err := r.q.GetPasskey(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return entity.Passkey{}, entity.ErrPasskeyNotFound
		}

		return entity.Passkey{}, fmt.Errorf("PasskeyRepo - GetByID - r.q.GetPasskey: %w", err)
	}

	return entity.Passkey(passkey), nil
}

// UpdateUsage replaces the stored credential record after a login (sign count, flags) and sets
// last_used_at.
func (r *Repo) UpdateUsage(ctx context.Context, id entity.PasskeyID, credential json.RawMessage, usedAt time.Time) error {
	n, err := r.q.UpdatePasskeyUsage(ctx, queries.UpdatePasskeyUsageParams{
		ID:         id,
		Credential: credential,
		LastUsedAt: &usedAt,
	})
	if err != nil {
		return fmt.Errorf("PasskeyRepo - UpdateUsage - r.q.UpdatePasskeyUsage: %w", err)
	}

	if n == 0 {
		return entity.ErrPasskeyNotFound
	}

	return nil
}

// Rename sets the name of the user's passkey id. A passkey that is not the user's is
// ErrPasskeyNotFound, the same as one that does not exist.
func (r *Repo) Rename(ctx context.Context, userID string, id entity.PasskeyID, name string) (entity.Passkey, error) {
	passkey, err := r.q.RenamePasskey(ctx, queries.RenamePasskeyParams{ID: id, UserID: userID, Name: name})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return entity.Passkey{}, entity.ErrPasskeyNotFound
		}

		return entity.Passkey{}, fmt.Errorf("PasskeyRepo - Rename - r.q.RenamePasskey: %w", err)
	}

	return entity.Passkey(passkey), nil
}

// Delete removes the user's passkey id. A passkey that is not the user's is ErrPasskeyNotFound,
// the same as one that does not exist.
func (r *Repo) Delete(ctx context.Context, userID string, id entity.PasskeyID) error {
	n, err := r.q.DeletePasskey(ctx, queries.DeletePasskeyParams{ID: id, UserID: userID})
	if err != nil {
		return fmt.Errorf("PasskeyRepo - Delete - r.q.DeletePasskey: %w", err)
	}

	if n == 0 {
		return entity.ErrPasskeyNotFound
	}

	return nil
}

// BurnChallenge records a login challenge as used until expiresAt. The insert is the check: a hash
// already present inserts no row, so two concurrent finishes of one ceremony cannot both succeed.
func (r *Repo) BurnChallenge(ctx context.Context, hash []byte, expiresAt time.Time) error {
	_, err := r.q.BurnChallenge(ctx, queries.BurnChallengeParams{ChallengeHash: hash, ExpiresAt: expiresAt})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return entity.ErrPasskeyChallengeUsed
		}

		return fmt.Errorf("PasskeyRepo - BurnChallenge - r.q.BurnChallenge: %w", err)
	}

	return nil
}

// DeleteExpiredChallenges forgets used challenges whose ceremonies have expired: a token that old
// is rejected before its challenge is ever looked up.
func (r *Repo) DeleteExpiredChallenges(ctx context.Context) error {
	err := r.q.DeleteExpiredChallenges(ctx)
	if err != nil {
		return fmt.Errorf("PasskeyRepo - DeleteExpiredChallenges - r.q.DeleteExpiredChallenges: %w", err)
	}

	return nil
}
