package app

import (
	"fmt"
	"strings"
	"time"

	"github.com/m7medvision/go-clean-template/config"
	"github.com/m7medvision/go-clean-template/internal/usecase/oidc/sso"
	"github.com/m7medvision/go-clean-template/pkg/jwt"
)

const (
	// _oidcStateScope scopes the SSO sign-in state token's key and audience.
	_oidcStateScope = "oidc-state"
	// _oidcStateTimeout is how long a user has to sign in at the provider.
	_oidcStateTimeout = 10 * time.Minute
	// _oidcCallbackPath is the web client's route the provider redirects back to.
	_oidcCallbackPath = "/sso/callback"
)

// newSSO builds the SSO sign-in the OIDC handlers share. It never fails on the provider: that is
// discovered on the first sign-in, so the app starts while it is down. With no OIDC_ISSUER it is off.
func newSSO(cfg *config.Config) (*sso.SSO, error) {
	state, err := jwt.NewScoped(cfg.JWT.Secret, _oidcStateScope, _oidcStateTimeout)
	if err != nil {
		return nil, fmt.Errorf("jwt.NewScoped: %w", err)
	}

	return sso.New(state, &sso.Config{
		Issuer:       cfg.OIDC.Issuer,
		ClientID:     cfg.OIDC.ClientID,
		ClientSecret: cfg.OIDC.ClientSecret,
		RedirectURL:  strings.TrimSuffix(cfg.App.BaseURL, "/") + _oidcCallbackPath,
		RolesClaim:   cfg.OIDC.RolesClaim,
		RoleMap:      cfg.OIDC.RoleMap,
	}), nil
}
