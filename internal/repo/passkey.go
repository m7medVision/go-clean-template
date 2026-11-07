package repo

import (
	"context"
	"encoding/json"
	"time"

	"github.com/m7medvision/go-clean-template/internal/entity"
)

// The command tests run the real ceremony and an in-memory guard, so only the read model has a mock.
//go:generate mockgen -source=passkey.go -destination=repomock/passkey.go -package=repomock -exclude_interfaces=PasskeyCeremony,ChallengeGuard

type (
	// PasskeyReadModel reads webauthn_credentials, which only the passkey projector writes.
	// ListByUser is the user's passkeys, oldest first. Get of a missing passkey is
	// entity.ErrPasskeyNotFound; whether the caller may read it is entity.Passkey's rule.
	PasskeyReadModel interface {
		ListByUser(ctx context.Context, userID string) ([]entity.Passkey, error)
		Get(ctx context.Context, id entity.PasskeyID) (entity.Passkey, error)
	}

	// PasskeyCeremony runs the WebAuthn ceremonies of passkey registration and sign-in. Callers pass
	// and get the JSON the browser speaks; a ceremony's state travels from begin to finish in its
	// token. It stores nothing. Anything that does not complete a ceremony is
	// entity.ErrPasskeyCeremonyInvalid.
	PasskeyCeremony interface {
		// CheckReauth refuses a sign-in at authTime too old to add a passkey, with
		// entity.ErrPasskeyReauthRequired.
		CheckReauth(authTime time.Time) error
		// BeginRegistration starts adding a passkey for user, excluding the passkeys they have.
		BeginRegistration(user *entity.User, stored []entity.Passkey) (entity.CeremonyStarted, error)
		// FinishRegistration verifies the browser's new credential against the ceremony in token,
		// begun for userID, and returns its ID and the record to store.
		FinishRegistration(userID, token string, body []byte) (entity.PasskeyID, json.RawMessage, error)
		// BeginLogin starts a sign-in with any passkey the authenticator holds for the app.
		BeginLogin() (entity.CeremonyStarted, error)
		// LoginFor reads the user and passkey an assertion names, before anything is verified.
		LoginFor(body []byte) (userID string, id entity.PasskeyID, err error)
		// FinishLogin verifies the assertion in body against the ceremony in token, for user's
		// passkey as stored, and returns the sign-in to record. A sign count that did not advance
		// (a possible clone) does not complete it.
		FinishLogin(token string, body []byte, user *entity.User, passkey *entity.Passkey) (entity.PasskeyLogin, error)
	}

	// ChallengeGuard keeps the login challenges already used, a write-side guard. Burn records a
	// challenge hash as used until expiresAt and runs then in the same transaction, which the guard
	// opens: the burn commits only with what then writes through the context it is given, so a
	// login's burn and its recorded use commit together or not at all. A hash burned before is
	// entity.ErrPasskeyChallengeUsed, and then does not run.
	ChallengeGuard interface {
		Burn(ctx context.Context, hash []byte, expiresAt time.Time, then func(ctx context.Context) error) error
	}
)
