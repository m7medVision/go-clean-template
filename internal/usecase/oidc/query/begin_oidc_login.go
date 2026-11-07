// Package query holds the OIDC domain's queries, one file each, with their handlers. Beginning a
// sign-in is a query: it records nothing, since the token it returns carries the ceremony's state.
package query

import (
	"context"
	"fmt"

	"github.com/m7medvision/go-clean-template/internal/repo"
	"github.com/m7medvision/go-clean-template/pkg/cqrs"
)

// BeginOIDCLogin returns the provider URL to send the browser to and the token FinishOIDCLogin
// needs. With no provider configured it is entity.ErrOIDCDisabled.
type BeginOIDCLogin struct {
	cqrs.Query[Started]
}

type Started struct {
	AuthURL string
	Token   string
}

type BeginOIDCLoginHandler struct {
	SSO repo.SSOProvider
}

func (h BeginOIDCLoginHandler) Handle(ctx context.Context, _ BeginOIDCLogin) (Started, error) {
	authURL, token, err := h.SSO.Begin(ctx)
	if err != nil {
		return Started{}, fmt.Errorf("BeginOIDCLogin: %w", err)
	}

	return Started{AuthURL: authURL, Token: token}, nil
}
