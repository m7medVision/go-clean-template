// Package query holds the passkey domain's queries, one file each, with their handlers. They read
// only the read models. The ceremony begin steps are queries: they record nothing, since the token
// they return carries the ceremony's state to its finish.
package query

import (
	"context"
	"fmt"
	"time"

	"github.com/m7medvision/go-clean-template/internal/repo"
	"github.com/m7medvision/go-clean-template/internal/usecase/passkey/ceremony"
)

// BeginPasskeyRegistration returns the creation options for navigator.credentials.create, excluding
// UserID's passkeys, and the token FinishPasskeyRegistration needs. AuthTime is when the caller's
// auth token was issued: one older than the reauth max age (or zero) is
// entity.ErrPasskeyReauthRequired.
type BeginPasskeyRegistration struct {
	UserID   string
	AuthTime time.Time
}

// QueryName -.
func (BeginPasskeyRegistration) QueryName() string { return "BeginPasskeyRegistration" }

// BeginPasskeyRegistrationHandler handles BeginPasskeyRegistration.
type BeginPasskeyRegistrationHandler struct {
	Ceremony *ceremony.Ceremony
	Users    repo.UserReadModel
	Passkeys repo.PasskeyReadModel
}

// Handle -.
func (h BeginPasskeyRegistrationHandler) Handle(ctx context.Context, q BeginPasskeyRegistration) (ceremony.Started, error) {
	// A stale sign-in is refused before anything is read.
	if err := h.Ceremony.CheckReauth(q.AuthTime); err != nil {
		return ceremony.Started{}, fmt.Errorf("BeginPasskeyRegistration: %w", err)
	}

	user, err := h.Users.ByID(ctx, q.UserID)
	if err != nil {
		return ceremony.Started{}, fmt.Errorf("BeginPasskeyRegistration - h.Users.ByID: %w", err)
	}

	stored, err := h.Passkeys.ListByUser(ctx, q.UserID)
	if err != nil {
		return ceremony.Started{}, fmt.Errorf("BeginPasskeyRegistration - h.Passkeys.ListByUser: %w", err)
	}

	started, err := h.Ceremony.BeginRegistration(&user, stored)
	if err != nil {
		return ceremony.Started{}, fmt.Errorf("BeginPasskeyRegistration: %w", err)
	}

	return started, nil
}
