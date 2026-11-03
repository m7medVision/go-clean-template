// Package usecase implements application business logic. Each logic group in own file.
package usecase

import (
	"context"

	"github.com/m7medvision/go-clean-template/internal/entity"
)

//go:generate mockgen -source=contracts.go -destination=./mocks_usecase_test.go -package=usecase_test

type (
	// OIDC signs users in through an OpenID Connect provider (authorization code with PKCE). Begin
	// returns the provider URL to send the browser to and a token carrying the ceremony state; the
	// provider redirects back with a code and the state, which Finish checks against the token
	// before it exchanges the code. Finish returns the same auth token password login issues, with
	// the roles the provider's claims map to. The first sign-in of an identity creates a user with
	// no password. With no provider configured, Begin and Finish are entity.ErrOIDCDisabled.
	OIDC interface {
		Enabled() bool
		Begin(ctx context.Context) (authURL, token string, err error)
		Finish(ctx context.Context, code, state, token string) (string, error)
	}

	// Mail delivers email. Use cases do not call it: they enqueue with repo.QueueRepo, and the
	// worker calls Deliver for each message on entity.EmailQueue.
	Mail interface {
		Deliver(ctx context.Context, email entity.Email) error
	}
)
