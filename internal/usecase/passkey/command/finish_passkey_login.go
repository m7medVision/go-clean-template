package command

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/m7medvision/go-clean-template/internal/entity"
	"github.com/m7medvision/go-clean-template/internal/repo"
	"github.com/m7medvision/go-clean-template/internal/usecase/aggregate"
	"github.com/m7medvision/go-clean-template/internal/usecase/passkey/ceremony"
	"github.com/m7medvision/go-clean-template/pkg/cqrs"
)

// FinishPasskeyLogin verifies the browser's assertion against the ceremony in Token, burns the
// ceremony's challenge, records the passkey's use, and returns the same auth token password login
// issues. Anything that does not complete the ceremony is entity.ErrPasskeyLoginInvalid; a login
// finished a second time is entity.ErrPasskeyChallengeUsed.
//
// The burn comes after the assertion verifies and before anything is recorded or issued: only a
// signature from a registered passkey can consume a challenge, so a captured token with a forged body
// cannot burn a ceremony its owner is still completing. The burn and the use commit together.
type FinishPasskeyLogin struct {
	cqrs.Command[entity.SignedIn]

	Token string
	Body  []byte
}

// FinishPasskeyLoginHandler handles FinishPasskeyLogin. Now is the clock (time.Now when nil).
type FinishPasskeyLoginHandler struct {
	Ceremony *ceremony.Ceremony
	Store    repo.EventStore
	Users    repo.UserReadModel
	Guard    repo.ChallengeGuard
	Tx       repo.Transactor
	Sessions repo.SessionIssuer
	Now      func() time.Time
}

// Handle -.
func (h *FinishPasskeyLoginHandler) Handle(ctx context.Context, cmd FinishPasskeyLogin) (entity.SignedIn, error) {
	var used *entity.PasskeyAggregate

	login, err := h.Ceremony.FinishLogin(cmd.Token, cmd.Body, h.lookup(ctx, &used))
	if errors.Is(err, entity.ErrPasskeyCeremonyInvalid) {
		return entity.SignedIn{}, fmt.Errorf("FinishPasskeyLogin: %w: %w", entity.ErrPasskeyLoginInvalid, err)
	}

	if err != nil {
		return entity.SignedIn{}, fmt.Errorf("FinishPasskeyLogin: %w", err)
	}

	written, err := h.record(ctx, &login, used)
	if err != nil {
		return entity.SignedIn{}, err
	}

	token, err := h.Sessions.Issue(login.UserID, entity.LocalRoles())
	if err != nil {
		return entity.SignedIn{}, fmt.Errorf("FinishPasskeyLogin - h.Sessions.Issue: %w", err)
	}

	return entity.SignedIn{Written: written, Token: token}, nil
}

// lookup finds the user and passkey an assertion names, and keeps the passkey's aggregate in used.
// The credential is loaded from its stream, not the read model, so the sign count it checks is the
// latest one even right after another login.
func (h *FinishPasskeyLoginHandler) lookup(ctx context.Context, used **entity.PasskeyAggregate) ceremony.Lookup {
	return func(userID string, id entity.PasskeyID) (*entity.User, *entity.Passkey, error) {
		user, err := h.Users.ByID(ctx, userID)
		if err != nil {
			return nil, nil, fmt.Errorf("h.Users.ByID: %w", err)
		}

		passkey := &entity.PasskeyAggregate{}

		err = h.Store.Load(ctx, entity.PasskeyStreamID(id), passkey)
		if errors.Is(err, entity.ErrStreamNotFound) || (err == nil && !passkey.OwnedBy(userID)) {
			return &user, nil, nil
		}

		if err != nil {
			return nil, nil, fmt.Errorf("h.Store.Load: %w", err)
		}

		*used = passkey

		return &user, &entity.Passkey{ID: id, UserID: userID, Credential: passkey.Credential()}, nil
	}
}

// record burns the login's challenge and records the passkey's use, in one transaction.
func (h *FinishPasskeyLoginHandler) record(ctx context.Context, login *ceremony.Login, used *entity.PasskeyAggregate) (
	entity.Written, error,
) {
	if used == nil {
		return entity.Written{}, fmt.Errorf("FinishPasskeyLogin - no passkey: %w", entity.ErrPasskeyLoginInvalid)
	}

	var written entity.Written

	err := h.Tx.InTx(ctx, func(ctx context.Context) error {
		if err := h.Guard.DeleteExpiredChallenges(ctx); err != nil {
			return fmt.Errorf("h.Guard.DeleteExpiredChallenges: %w", err)
		}

		if err := h.Guard.BurnChallenge(ctx, login.ChallengeHash, login.ExpiresAt); err != nil {
			return fmt.Errorf("h.Guard.BurnChallenge: %w", err)
		}

		if err := used.Use(login.UserID, login.Credential, aggregate.Now(h.Now)); err != nil {
			return err
		}

		var err error

		written, err = aggregate.Save(ctx, h.Store, used)

		return err
	})
	if err != nil {
		return entity.Written{}, fmt.Errorf("FinishPasskeyLogin: %w", err)
	}

	return written, nil
}
