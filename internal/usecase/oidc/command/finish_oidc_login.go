// Package command holds the OIDC domain's command, with its handler.
package command

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/m7medvision/go-clean-template/internal/entity"
	"github.com/m7medvision/go-clean-template/internal/repo"
	"github.com/m7medvision/go-clean-template/internal/usecase/aggregate"
	"github.com/m7medvision/go-clean-template/pkg/cqrs"
)

// FinishOIDCLogin checks the provider's redirect (Code and State) against the ceremony in Token and
// signs in as the user linked to the identity, creating one with no password on its first sign-in.
// It returns the same auth token password login issues, with the roles the provider's claims map to.
//
// The identity's user is found through its claim, held on the write side, so the second sign-in of
// an identity finds its user even before the user read model has it. An email or username another
// account holds is entity.ErrAccountConflict: accounts are never linked by email.
type FinishOIDCLogin struct {
	cqrs.Command[entity.SignedIn]

	Code  string
	State string
	Token string
}

// FinishOIDCLoginHandler's Now is the clock (time.Now when nil).
type FinishOIDCLoginHandler struct {
	SSO      repo.SSOProvider
	Store    repo.EventStore
	Sessions repo.SessionIssuer
	Now      func() time.Time
}

func (h FinishOIDCLoginHandler) Handle(ctx context.Context, cmd FinishOIDCLogin) (entity.SignedIn, error) {
	identity, err := h.SSO.Verify(ctx, cmd.Code, cmd.State, cmd.Token)
	if err != nil {
		return entity.SignedIn{}, fmt.Errorf("FinishOIDCLogin - h.SSO.Verify: %w", err)
	}

	written, err := h.user(ctx, &identity)
	if err != nil {
		return entity.SignedIn{}, fmt.Errorf("FinishOIDCLogin: %w", err)
	}

	token, err := h.Sessions.Issue(written.ID, identity.Roles)
	if err != nil {
		return entity.SignedIn{}, fmt.Errorf("FinishOIDCLogin - h.Sessions.Issue: %w", err)
	}

	return entity.SignedIn{Written: written, Token: token}, nil
}

func (h FinishOIDCLoginHandler) user(ctx context.Context, identity *entity.SSOIdentity) (entity.Written, error) {
	claim := entity.OIDCIdentity(identity.Issuer, identity.Subject)

	owner, err := h.Store.ClaimOwner(ctx, entity.ClaimOIDCIdentity, claim)
	if err == nil {
		return entity.Written{ID: owner}, nil
	}

	if !errors.Is(err, entity.ErrClaimNotFound) {
		return entity.Written{}, fmt.Errorf("h.Store.ClaimOwner: %w", err)
	}

	id := uuid.NewString()
	user, claims := entity.NewOIDCUserAggregate(id, identity.Username, identity.Email, identity.Issuer, identity.Subject,
		aggregate.Now(h.Now))

	written, err := aggregate.Save(ctx, h.Store, user, claims...)
	if errors.Is(err, entity.ErrOIDCIdentityTaken) {
		// A concurrent first sign-in of the same identity created its user first.
		owner, err = h.Store.ClaimOwner(ctx, entity.ClaimOIDCIdentity, claim)
		if err != nil {
			return entity.Written{}, fmt.Errorf("h.Store.ClaimOwner after a race: %w", err)
		}

		return entity.Written{ID: owner}, nil
	}

	return written, err
}
