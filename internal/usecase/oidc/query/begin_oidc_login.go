// Package query holds the OIDC domain's queries, one file each, with their handlers. Beginning a
// sign-in is a query: it records nothing, since the token it returns carries the ceremony's state.
package query

import (
	"context"
	"fmt"

	"github.com/m7medvision/go-clean-template/internal/usecase/oidc/sso"
)

// BeginOIDCLogin returns the provider URL to send the browser to and the token FinishOIDCLogin
// needs. With no provider configured it is entity.ErrOIDCDisabled.
type BeginOIDCLogin struct{}

// QueryName -.
func (BeginOIDCLogin) QueryName() string { return "BeginOIDCLogin" }

// Started is a sign-in begun.
type Started struct {
	AuthURL string
	Token   string
}

// BeginOIDCLoginHandler handles BeginOIDCLogin.
type BeginOIDCLoginHandler struct {
	SSO *sso.SSO
}

// Handle -.
func (h BeginOIDCLoginHandler) Handle(ctx context.Context, _ BeginOIDCLogin) (Started, error) {
	authURL, token, err := h.SSO.Begin(ctx)
	if err != nil {
		return Started{}, fmt.Errorf("BeginOIDCLogin: %w", err)
	}

	return Started{AuthURL: authURL, Token: token}, nil
}
