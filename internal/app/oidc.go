package app

import (
	"github.com/m7medvision/go-clean-template/config"
	"github.com/m7medvision/go-clean-template/internal/repo/sso"
)

// newSSO builds the SSO sign-in the OIDC handlers share. It never fails on the provider: that is
// discovered on the first sign-in, so the app starts while it is down. With no OIDC_ISSUER it is off.
func newSSO(cfg *config.Config) (*sso.SSO, error) {
	return sso.New(cfg.JWT.Secret, &sso.Config{
		Issuer:       cfg.OIDC.Issuer,
		ClientID:     cfg.OIDC.ClientID,
		ClientSecret: cfg.OIDC.ClientSecret,
		BaseURL:      cfg.App.BaseURL,
		RolesClaim:   cfg.OIDC.RolesClaim,
		RoleMap:      cfg.OIDC.RoleMap,
	})
}
