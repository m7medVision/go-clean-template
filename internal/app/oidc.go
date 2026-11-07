package app

import (
	"fmt"

	restoidc "github.com/m7medvision/go-clean-template/internal/controller/restapi/v1/oidc"
	"github.com/m7medvision/go-clean-template/internal/repo/session"
	"github.com/m7medvision/go-clean-template/internal/repo/sso"
	"github.com/m7medvision/go-clean-template/internal/usecase/oidc"
	"github.com/m7medvision/go-clean-template/pkg/cqrs"
)

// oidcModule is SSO sign-in through an OpenID Connect provider, and its REST operations. It has no
// read model: a sign-in finds its user through the identity's claim.
func oidcModule() module {
	return module{
		handlers: func(d *deps, b cqrs.Buses) error {
			// The provider is discovered on the first sign-in, so the app starts while it is down.
			s, err := sso.New(d.cfg.JWT.Secret, d.cfg.OIDC, d.cfg.App.SSOCallbackURL())
			if err != nil {
				return fmt.Errorf("oidc - sso.New: %w", err)
			}

			oidc.Register(b.Commands, b.Queries, oidc.Deps{SSO: s, Store: d.store, Sessions: session.New(d.tokens)})

			return nil
		},
		rest: restoidc.NewRoutes,
	}
}
