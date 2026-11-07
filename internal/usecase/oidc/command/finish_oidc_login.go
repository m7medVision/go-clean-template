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
	"github.com/m7medvision/go-clean-template/internal/usecase/oidc/sso"
	"github.com/m7medvision/go-clean-template/pkg/cqrs"
	"github.com/m7medvision/go-clean-template/pkg/jwt"
)

// FinishOIDCLogin checks the provider's redirect (Code and State) against the ceremony in Token and
// signs in as the user linked to the identity, creating one with no password on its first sign-in.
// It returns the same auth token password login issues, with the roles the provider's claims map to.
//
// The identity's user is found through its claim, held on the write side, so the second sign-in of
// an identity finds its user even before the user read model has it. An email or username another
// account holds is entity.ErrAccountConflict: accounts are never linked by email.
type FinishOIDCLogin struct {
	cqrs.Command[SignedIn]

	Code  string
	State string
	Token string
}

// SignedIn is a sign-in's result: the user, the position of their creation when this sign-in created
// them, and the auth token.
type SignedIn struct {
	entity.Written

	Token string
}

// FinishOIDCLoginHandler handles FinishOIDCLogin. Now is the clock (time.Now when nil).
type FinishOIDCLoginHandler struct {
	SSO    *sso.SSO
	Store  repo.EventStore
	Tokens *jwt.Manager
	Now    func() time.Time
}

// Handle -.
func (h FinishOIDCLoginHandler) Handle(ctx context.Context, cmd FinishOIDCLogin) (SignedIn, error) {
	identity, err := h.SSO.Verify(ctx, cmd.Code, cmd.State, cmd.Token)
	if err != nil {
		return SignedIn{}, fmt.Errorf("FinishOIDCLogin - h.SSO.Verify: %w", err)
	}

	written, err := h.user(ctx, &identity)
	if err != nil {
		return SignedIn{}, fmt.Errorf("FinishOIDCLogin: %w", err)
	}

	token, err := h.Tokens.GenerateToken(written.ID, entity.RoleNames(identity.Roles))
	if err != nil {
		return SignedIn{}, fmt.Errorf("FinishOIDCLogin - h.Tokens.GenerateToken: %w", err)
	}

	return SignedIn{Written: written, Token: token}, nil
}

// user returns the user the identity's claim names, or creates one.
func (h FinishOIDCLoginHandler) user(ctx context.Context, identity *sso.Identity) (entity.Written, error) {
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

	written, err := aggregate.Create(ctx, h.Store, id, user, claims...)
	if errors.Is(err, entity.ErrUserAlreadyExists) {
		// A concurrent first sign-in of the same identity created its user first.
		owner, err = h.Store.ClaimOwner(ctx, entity.ClaimOIDCIdentity, claim)
		if err != nil {
			return entity.Written{}, fmt.Errorf("h.Store.ClaimOwner after a race: %w", err)
		}

		return entity.Written{ID: owner}, nil
	}

	return written, err
}
