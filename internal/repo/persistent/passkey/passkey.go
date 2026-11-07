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

// Repo reads webauthn_credentials. Its queries join the transaction the context carries.
type Repo struct {
	pg *postgres.Postgres
}

func NewReadModel(pg *postgres.Postgres) repo.PasskeyReadModel {
	return &Repo{pg: pg}
}

func (r *Repo) q(ctx context.Context) *queries.Queries {
	return queries.New(r.pg.Executor(ctx))
}

// Get returns the user's passkey id. Another user's passkey is ErrPasskeyNotFound, the same as one
// that does not exist.
func (r *Repo) Get(ctx context.Context, userID string, id entity.PasskeyID) (entity.Passkey, error) {
	passkey, err := r.q(ctx).GetPasskey(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) || (err == nil && passkey.UserID != userID) {
		return entity.Passkey{}, entity.ErrPasskeyNotFound
	}

	if err != nil {
		return entity.Passkey{}, fmt.Errorf("PasskeyReadModel - Get - r.q(ctx).GetPasskey: %w", err)
	}

	return entity.Passkey(passkey), nil
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

// Guard keeps the used login challenges in webauthn_used_challenges.
type Guard struct {
	pg *postgres.Postgres
}

func NewGuard(pg *postgres.Postgres) repo.ChallengeGuard {
	return &Guard{pg: pg}
}

// Burn forgets the challenges whose ceremonies have expired (a token that old is rejected before
// its challenge is ever looked up), records hash as used until expiresAt, and runs then, in one
// transaction. The insert is the check: a hash already present inserts no row, so two concurrent
// finishes of one ceremony cannot both succeed.
func (g *Guard) Burn(ctx context.Context, hash []byte, expiresAt time.Time, then func(ctx context.Context) error) error {
	return g.pg.InTx(ctx, func(ctx context.Context) error {
		q := queries.New(g.pg.Executor(ctx))

		if err := q.DeleteExpiredChallenges(ctx); err != nil {
			return fmt.Errorf("ChallengeGuard - Burn - q.DeleteExpiredChallenges: %w", err)
		}

		_, err := q.BurnChallenge(ctx, queries.BurnChallengeParams{ChallengeHash: hash, ExpiresAt: expiresAt})
		if errors.Is(err, pgx.ErrNoRows) {
			return entity.ErrPasskeyChallengeUsed
		}

		if err != nil {
			return fmt.Errorf("ChallengeGuard - Burn - q.BurnChallenge: %w", err)
		}

		return then(ctx)
	})
}
