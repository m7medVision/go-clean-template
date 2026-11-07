// Package query holds the passkey domain's queries, one file each, with their handlers. They read
// only the read models. The ceremony begin steps are queries: they record nothing, since the token
// they return carries the ceremony's state to its finish.
package query

import (
	"context"
	"fmt"
	"time"

	"github.com/m7medvision/go-clean-template/internal/entity"
	"github.com/m7medvision/go-clean-template/internal/repo"
	"github.com/m7medvision/go-clean-template/pkg/cqrs"
)

// BeginPasskeyRegistration returns the creation options for navigator.credentials.create, excluding
// UserID's passkeys, and the token FinishPasskeyRegistration needs. AuthTime is when the caller's
// auth token was issued: one older than the reauth max age (or zero) is
// entity.ErrPasskeyReauthRequired.
type BeginPasskeyRegistration struct {
	cqrs.Query[entity.CeremonyStarted]

	UserID   string
	AuthTime time.Time
}

type BeginPasskeyRegistrationHandler struct {
	Ceremony repo.PasskeyCeremony
	Users    repo.UserReadModel
	Passkeys repo.PasskeyReadModel
}

func (h BeginPasskeyRegistrationHandler) Handle(ctx context.Context, q BeginPasskeyRegistration) (entity.CeremonyStarted, error) {
	if err := h.Ceremony.CheckReauth(q.AuthTime); err != nil {
		return entity.CeremonyStarted{}, fmt.Errorf("BeginPasskeyRegistration: %w", err)
	}

	user, err := h.Users.ByID(ctx, q.UserID)
	if err != nil {
		return entity.CeremonyStarted{}, fmt.Errorf("BeginPasskeyRegistration - h.Users.ByID: %w", err)
	}

	stored, err := h.Passkeys.ListByUser(ctx, q.UserID)
	if err != nil {
		return entity.CeremonyStarted{}, fmt.Errorf("BeginPasskeyRegistration - h.Passkeys.ListByUser: %w", err)
	}

	started, err := h.Ceremony.BeginRegistration(&user, stored)
	if err != nil {
		return entity.CeremonyStarted{}, fmt.Errorf("BeginPasskeyRegistration: %w", err)
	}

	return started, nil
}
