// Package passkey implements the passkey read model on Postgres (webauthn_credentials), the
// used-challenge guard, the projector that writes the read model from the passkey events, and the
// importer that turns rows stored before passkeys stored events into events.
package passkey

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/m7medvision/go-clean-template/internal/entity"
	"github.com/m7medvision/go-clean-template/internal/repo"
	"github.com/m7medvision/go-clean-template/internal/repo/persistent/queries"
	"github.com/m7medvision/go-clean-template/pkg/postgres"
)

// Repo reads webauthn_credentials and keeps the used login challenges. Its queries join the
// transaction the context carries, so a burn commits with the login's events.
type Repo struct {
	pg *postgres.Postgres
}

// NewReadModel returns the passkey read model.
func NewReadModel(pg *postgres.Postgres) repo.PasskeyReadModel {
	return &Repo{pg: pg}
}

// NewGuard returns the used-challenge guard.
func NewGuard(pg *postgres.Postgres) repo.ChallengeGuard {
	return &Repo{pg: pg}
}

func (r *Repo) q(ctx context.Context) *queries.Queries {
	return queries.New(r.pg.Executor(ctx))
}

// Get returns the user's passkey id. Another user's passkey is ErrPasskeyNotFound, the same as one
// that does not exist.
func (r *Repo) Get(ctx context.Context, userID string, id entity.PasskeyID) (entity.Passkey, error) {
	passkey, err := r.GetByID(ctx, id)
	if err != nil {
		return entity.Passkey{}, err
	}

	if passkey.UserID != userID {
		return entity.Passkey{}, entity.ErrPasskeyNotFound
	}

	return passkey, nil
}

// ListByUser returns the user's passkeys, oldest first.
func (r *Repo) ListByUser(ctx context.Context, userID string) ([]entity.Passkey, error) {
	rows, err := r.q(ctx).ListPasskeysByUser(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("PasskeyReadModel - ListByUser - r.q(ctx).ListPasskeysByUser: %w", err)
	}

	passkeys := make([]entity.Passkey, len(rows))
	for i := range rows {
		passkeys[i] = entity.Passkey(rows[i])
	}

	return passkeys, nil
}

// GetByID -.
func (r *Repo) GetByID(ctx context.Context, id entity.PasskeyID) (entity.Passkey, error) {
	passkey, err := r.q(ctx).GetPasskey(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return entity.Passkey{}, entity.ErrPasskeyNotFound
		}

		return entity.Passkey{}, fmt.Errorf("PasskeyReadModel - GetByID - r.q(ctx).GetPasskey: %w", err)
	}

	return entity.Passkey(passkey), nil
}

// BurnChallenge records a login challenge as used until expiresAt. The insert is the check: a hash
// already present inserts no row, so two concurrent finishes of one ceremony cannot both succeed.
func (r *Repo) BurnChallenge(ctx context.Context, hash []byte, expiresAt time.Time) error {
	_, err := r.q(ctx).BurnChallenge(ctx, queries.BurnChallengeParams{ChallengeHash: hash, ExpiresAt: expiresAt})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return entity.ErrPasskeyChallengeUsed
		}

		return fmt.Errorf("PasskeyReadModel - BurnChallenge - r.q(ctx).BurnChallenge: %w", err)
	}

	return nil
}

// DeleteExpiredChallenges forgets used challenges whose ceremonies have expired: a token that old
// is rejected before its challenge is ever looked up.
func (r *Repo) DeleteExpiredChallenges(ctx context.Context) error {
	err := r.q(ctx).DeleteExpiredChallenges(ctx)
	if err != nil {
		return fmt.Errorf("PasskeyReadModel - DeleteExpiredChallenges - r.q(ctx).DeleteExpiredChallenges: %w", err)
	}

	return nil
}
