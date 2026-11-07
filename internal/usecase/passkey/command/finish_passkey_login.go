package command

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/m7medvision/go-clean-template/internal/entity"
	"github.com/m7medvision/go-clean-template/internal/repo"
	"github.com/m7medvision/go-clean-template/internal/usecase/aggregate"
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

// FinishPasskeyLoginHandler's Now is the clock (time.Now when nil).
type FinishPasskeyLoginHandler struct {
	Ceremony repo.PasskeyCeremony
	Store    repo.EventStore
	Users    repo.UserReadModel
	Guard    repo.ChallengeGuard
	Sessions repo.SessionIssuer
	Now      func() time.Time
}

func (h *FinishPasskeyLoginHandler) Handle(ctx context.Context, cmd FinishPasskeyLogin) (entity.SignedIn, error) {
	userID, id, err := h.Ceremony.LoginFor(cmd.Body)
	if err != nil {
		return entity.SignedIn{}, loginFailed(err)
	}

	user, passkey, err := h.find(ctx, userID, id)
	if err != nil {
		return entity.SignedIn{}, loginFailed(err)
	}

	login, err := h.Ceremony.FinishLogin(cmd.Token, cmd.Body, &user,
		&entity.Passkey{ID: id, UserID: userID, Credential: passkey.Credential()})
	if err != nil {
		return entity.SignedIn{}, loginFailed(err)
	}

	written, err := h.record(ctx, &login, passkey)
	if err != nil {
		return entity.SignedIn{}, fmt.Errorf("FinishPasskeyLogin: %w", err)
	}

	token, err := h.Sessions.Issue(login.UserID, entity.LocalRoles())
	if err != nil {
		return entity.SignedIn{}, fmt.Errorf("FinishPasskeyLogin - h.Sessions.Issue: %w", err)
	}

	return entity.SignedIn{Written: written, Token: token}, nil
}

// find loads the user an assertion names and their passkey. The passkey is loaded from its stream,
// not the read model, so the sign count it is checked against is the latest one, even right after
// another sign-in. An unknown user, or a passkey that is unknown or not theirs, does not complete
// the ceremony.
func (h *FinishPasskeyLoginHandler) find(ctx context.Context, userID string, id entity.PasskeyID) (
	entity.User, *entity.PasskeyAggregate, error,
) {
	user, err := h.Users.ByID(ctx, userID)
	if errors.Is(err, entity.ErrUserNotFound) {
		return entity.User{}, nil, fmt.Errorf("%w: %w", entity.ErrPasskeyCeremonyInvalid, err)
	}

	if err != nil {
		return entity.User{}, nil, fmt.Errorf("h.Users.ByID: %w", err)
	}

	passkey, err := aggregate.Load[entity.PasskeyAggregate](ctx, h.Store, id.String())
	if err == nil {
		err = passkey.OwnedBy(userID)
	}

	if errors.Is(err, entity.ErrPasskeyNotFound) {
		return entity.User{}, nil, fmt.Errorf("no such passkey: %w", entity.ErrPasskeyCeremonyInvalid)
	}

	if err != nil {
		return entity.User{}, nil, fmt.Errorf("aggregate.Load: %w", err)
	}

	return user, passkey, nil
}

// loginFailed says that a ceremony that did not complete is a failed sign-in.
func loginFailed(err error) error {
	if errors.Is(err, entity.ErrPasskeyCeremonyInvalid) {
		return fmt.Errorf("FinishPasskeyLogin: %w: %w", entity.ErrPasskeyLoginInvalid, err)
	}

	return fmt.Errorf("FinishPasskeyLogin: %w", err)
}

// record burns the login's challenge and records the passkey's use: the guard saves the use in the
// burn's transaction, so both commit or neither does.
func (h *FinishPasskeyLoginHandler) record(
	ctx context.Context, login *entity.PasskeyLogin, passkey *entity.PasskeyAggregate,
) (entity.Written, error) {
	var written entity.Written

	err := h.Guard.Burn(ctx, login.ChallengeHash, login.ExpiresAt, func(ctx context.Context) error {
		if err := passkey.Use(login.UserID, login.Credential, aggregate.Now(h.Now)); err != nil {
			return err
		}

		var err error

		written, err = aggregate.Save(ctx, h.Store, passkey)

		return err
	})
	if err != nil {
		return entity.Written{}, fmt.Errorf("h.Guard.Burn: %w", err)
	}

	return written, nil
}
