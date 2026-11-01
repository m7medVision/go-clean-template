// Package usecase implements application business logic. Each logic group in own file.
package usecase

import (
	"context"
	"time"

	"github.com/m7medvision/go-clean-template/internal/entity"
	"github.com/shopspring/decimal"
)

//go:generate mockgen -source=contracts.go -destination=./mocks_usecase_test.go -package=usecase_test

type (
	// Exchange converts amounts between currencies and keeps each user's history.
	Exchange interface {
		Convert(ctx context.Context, userID, from, to string, amount decimal.Decimal) (entity.Conversion, error)
		History(ctx context.Context, userID string, limit, offset int) (entity.ExchangeHistory, error)
	}

	// User -.
	User interface {
		Register(ctx context.Context, username, email, password string) (entity.User, error)
		Login(ctx context.Context, email, password string) (string, error)
		GetUser(ctx context.Context, userID string) (entity.User, error)
	}

	// Task -.
	Task interface {
		Create(ctx context.Context, userID, title, description string) (entity.Task, error)
		Get(ctx context.Context, userID, taskID string) (entity.Task, error)
		List(ctx context.Context, userID string, status *entity.TaskStatus, limit, offset int) ([]entity.Task, int, error)
		// ListAll is List across every user, for a caller holding entity.RoleAdmin; anyone else gets
		// entity.ErrAdminRequired.
		ListAll(
			ctx context.Context, caller entity.Principal, status *entity.TaskStatus, limit, offset int,
		) ([]entity.Task, int, error)
		Update(ctx context.Context, userID, taskID, title, description string) (entity.Task, error)
		Transition(ctx context.Context, userID, taskID string, newStatus entity.TaskStatus) (entity.Task, error)
		Delete(ctx context.Context, userID, taskID string) error
	}

	// Passkey registers, lists, renames and deletes users' WebAuthn credentials, and signs users in
	// with them. Options and credential bodies are the WebAuthn JSON the browser reads and writes;
	// the token carries the ceremony state between begin and finish. A token or body that does not
	// complete the ceremony is entity.ErrPasskeyCeremonyInvalid; a login finished a second time is
	// entity.ErrPasskeyChallengeUsed.
	Passkey interface {
		// BeginRegistration needs a recent sign-in: authTime is when the caller's auth token was
		// issued, and one older than the configured max age (or zero) is
		// entity.ErrPasskeyReauthRequired.
		BeginRegistration(ctx context.Context, userID string, authTime time.Time) (options []byte, token string, err error)
		FinishRegistration(ctx context.Context, userID, token, name string, body []byte) (entity.Passkey, error)
		List(ctx context.Context, userID string) ([]entity.Passkey, error)
		// Rename and Delete act on the user's own passkey id (base64url). Any other id, including
		// another user's passkey, is entity.ErrPasskeyNotFound.
		Rename(ctx context.Context, userID, id, name string) (entity.Passkey, error)
		Delete(ctx context.Context, userID, id string) error
		BeginLogin(ctx context.Context) (options []byte, token string, err error)
		// FinishLogin returns the same auth token password login issues.
		FinishLogin(ctx context.Context, token string, body []byte) (string, error)
	}

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
