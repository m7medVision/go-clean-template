// Package passkey registers WebAuthn credentials and signs users in with them. go-webauthn stays
// inside this package: callers pass and get the WebAuthn JSON the browser speaks, and the ceremony
// state travels in a signed token. A login challenge is burned on finish, so it completes once.
package passkey

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/go-webauthn/webauthn/protocol"
	"github.com/go-webauthn/webauthn/webauthn"
	"github.com/google/uuid"
	"github.com/m7medvision/go-clean-template/internal/entity"
	"github.com/m7medvision/go-clean-template/internal/repo"
	"github.com/m7medvision/go-clean-template/internal/usecase"
	"github.com/m7medvision/go-clean-template/pkg/jwt"
)

// _defaultName names a passkey registered without one.
const _defaultName = "Passkey"

// Config is the relying party: who passkeys are registered with, and where from.
type Config struct {
	RPID      string
	RPName    string
	RPOrigins []string
	// Timeout is the ceremony's lifetime, sent to the browser and enforced on finish.
	Timeout time.Duration
	// ReauthMaxAge is how recent a sign-in must be to add a passkey.
	ReauthMaxAge time.Duration
	// Now is the clock the sign-in age is measured against; nil is time.Now.
	Now func() time.Time
}

// UseCase -.
type UseCase struct {
	users    repo.UserRepo
	passkeys repo.PasskeyRepo
	ceremony *jwt.Scoped
	tokens   *jwt.Manager
	webauthn *webauthn.WebAuthn
	timeout  time.Duration
	reauth   time.Duration
	now      func() time.Time
}

// New returns a Passkey usecase instrumented with OpenTelemetry tracing spans. ceremony signs the
// state a ceremony carries from begin to finish; its lifetime should match cfg.Timeout. tokens
// issues the auth token a passkey login returns, the same one password login issues.
func New(
	users repo.UserRepo, passkeys repo.PasskeyRepo, ceremony *jwt.Scoped, tokens *jwt.Manager, cfg *Config,
) (usecase.Passkey, error) {
	w, err := webauthn.New(&webauthn.Config{
		RPID:                  cfg.RPID,
		RPDisplayName:         cfg.RPName,
		RPOrigins:             cfg.RPOrigins,
		AttestationPreference: protocol.PreferNoAttestation,
		AuthenticatorSelection: protocol.AuthenticatorSelection{
			ResidentKey:        protocol.ResidentKeyRequirementRequired,
			RequireResidentKey: protocol.ResidentKeyRequired(),
			UserVerification:   protocol.VerificationRequired,
		},
		Timeouts: webauthn.TimeoutsConfig{
			Login:        webauthn.TimeoutConfig{Enforce: true, Timeout: cfg.Timeout, TimeoutUVD: cfg.Timeout},
			Registration: webauthn.TimeoutConfig{Enforce: true, Timeout: cfg.Timeout, TimeoutUVD: cfg.Timeout},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("PasskeyUseCase - New - webauthn.New: %w", err)
	}

	now := cfg.Now
	if now == nil {
		now = time.Now
	}

	return newTraced(&UseCase{
		users:    users,
		passkeys: passkeys,
		ceremony: ceremony,
		tokens:   tokens,
		webauthn: w,
		timeout:  cfg.Timeout,
		reauth:   cfg.ReauthMaxAge,
		now:      now,
	}), nil
}

// BeginRegistration returns the creation options for navigator.credentials.create, excluding the
// user's registered passkeys, and the token FinishRegistration needs.
//
// A passkey outlives the session that adds it and any password change, so adding one needs a recent
// sign-in: authTime, when the caller's auth token was issued, must be within the reauth max age. A
// stolen token past that age cannot plant a passkey. A zero authTime (a token without one) is too old.
func (uc *UseCase) BeginRegistration(
	ctx context.Context, userID string, authTime time.Time,
) (options []byte, token string, err error) {
	if authTime.IsZero() || uc.now().Sub(authTime) > uc.reauth {
		return nil, "", fmt.Errorf("PasskeyUseCase - BeginRegistration - signed in at %s: %w",
			authTime.UTC().Format(time.RFC3339), entity.ErrPasskeyReauthRequired)
	}

	user, err := uc.users.GetByID(ctx, userID)
	if err != nil {
		return nil, "", fmt.Errorf("PasskeyUseCase - BeginRegistration - uc.users.GetByID: %w", err)
	}

	stored, err := uc.passkeys.ListByUser(ctx, userID)
	if err != nil {
		return nil, "", fmt.Errorf("PasskeyUseCase - BeginRegistration - uc.passkeys.ListByUser: %w", err)
	}

	account, err := newAccount(&user, stored)
	if err != nil {
		return nil, "", fmt.Errorf("PasskeyUseCase - BeginRegistration - newAccount: %w", err)
	}

	creation, session, err := uc.webauthn.BeginRegistration(account,
		webauthn.WithExclusions(webauthn.Credentials(account.credentials).CredentialDescriptors()))
	if err != nil {
		return nil, "", fmt.Errorf("PasskeyUseCase - BeginRegistration - uc.webauthn.BeginRegistration: %w", err)
	}

	options, err = json.Marshal(creation.Response)
	if err != nil {
		return nil, "", fmt.Errorf("PasskeyUseCase - BeginRegistration - json.Marshal options: %w", err)
	}

	token, err = uc.signSession(session)
	if err != nil {
		return nil, "", fmt.Errorf("PasskeyUseCase - BeginRegistration - uc.signSession: %w", err)
	}

	return options, token, nil
}

// FinishRegistration verifies the browser's credential (PublicKeyCredential.toJSON) against the
// ceremony in token and stores it as a passkey named name.
func (uc *UseCase) FinishRegistration(ctx context.Context, userID, token, name string, body []byte) (entity.Passkey, error) {
	session, err := uc.verifySession(token)
	if err != nil {
		return entity.Passkey{}, fmt.Errorf("PasskeyUseCase - FinishRegistration - uc.verifySession: %w", err)
	}

	handle, err := userHandle(userID)
	if err != nil {
		return entity.Passkey{}, fmt.Errorf("PasskeyUseCase - FinishRegistration - userHandle: %w", err)
	}

	parsed, err := protocol.ParseCredentialCreationResponseBody(bytes.NewReader(body))
	if err != nil {
		return entity.Passkey{}, fmt.Errorf("PasskeyUseCase - FinishRegistration - protocol.ParseCredentialCreationResponseBody: %w: %w",
			entity.ErrPasskeyCeremonyInvalid, err)
	}

	// The account's handle must match the one the session was begun for; CreateCredential checks it.
	credential, err := uc.webauthn.CreateCredential(&account{handle: handle}, session, parsed)
	if err != nil {
		return entity.Passkey{}, fmt.Errorf("PasskeyUseCase - FinishRegistration - uc.webauthn.CreateCredential: %w: %w",
			entity.ErrPasskeyCeremonyInvalid, err)
	}

	record, err := json.Marshal(credential)
	if err != nil {
		return entity.Passkey{}, fmt.Errorf("PasskeyUseCase - FinishRegistration - json.Marshal credential: %w", err)
	}

	passkey := entity.Passkey{
		ID:         credential.ID,
		UserID:     userID,
		Name:       passkeyName(name),
		Credential: record,
		CreatedAt:  time.Now().UTC(),
	}

	err = uc.passkeys.Store(ctx, &passkey)
	if err != nil {
		return entity.Passkey{}, fmt.Errorf("PasskeyUseCase - FinishRegistration - uc.passkeys.Store: %w", err)
	}

	return passkey, nil
}

// List returns the user's passkeys, oldest first.
func (uc *UseCase) List(ctx context.Context, userID string) ([]entity.Passkey, error) {
	passkeys, err := uc.passkeys.ListByUser(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("PasskeyUseCase - List - uc.passkeys.ListByUser: %w", err)
	}

	return passkeys, nil
}

// Rename names the user's passkey id. The name is trimmed, and a blank one gets the default, as on
// registration.
func (uc *UseCase) Rename(ctx context.Context, userID, id, name string) (entity.Passkey, error) {
	var passkeyID entity.PasskeyID

	err := passkeyID.UnmarshalText([]byte(id))
	if err != nil {
		return entity.Passkey{}, fmt.Errorf("PasskeyUseCase - Rename - passkeyID.UnmarshalText: %w", err)
	}

	passkey, err := uc.passkeys.Rename(ctx, userID, passkeyID, passkeyName(name))
	if err != nil {
		return entity.Passkey{}, fmt.Errorf("PasskeyUseCase - Rename - uc.passkeys.Rename: %w", err)
	}

	return passkey, nil
}

// Delete removes the user's passkey id; it can no longer sign in. Deleting the last passkey is
// allowed: every account still has its password.
func (uc *UseCase) Delete(ctx context.Context, userID, id string) error {
	var passkeyID entity.PasskeyID

	err := passkeyID.UnmarshalText([]byte(id))
	if err != nil {
		return fmt.Errorf("PasskeyUseCase - Delete - passkeyID.UnmarshalText: %w", err)
	}

	err = uc.passkeys.Delete(ctx, userID, passkeyID)
	if err != nil {
		return fmt.Errorf("PasskeyUseCase - Delete - uc.passkeys.Delete: %w", err)
	}

	return nil
}

// passkeyName trims a passkey's name; a blank one is the default.
func passkeyName(name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return _defaultName
	}

	return name
}

// BeginLogin returns the request options for a discoverable navigator.credentials.get (no
// allowCredentials: the authenticator offers its passkeys for this RP) and the token FinishLogin needs.
// Nothing is stored.
func (uc *UseCase) BeginLogin(_ context.Context) (options []byte, token string, err error) {
	assertion, session, err := uc.webauthn.BeginDiscoverableLogin()
	if err != nil {
		return nil, "", fmt.Errorf("PasskeyUseCase - BeginLogin - uc.webauthn.BeginDiscoverableLogin: %w", err)
	}

	options, err = json.Marshal(assertion.Response)
	if err != nil {
		return nil, "", fmt.Errorf("PasskeyUseCase - BeginLogin - json.Marshal options: %w", err)
	}

	token, err = uc.signSession(session)
	if err != nil {
		return nil, "", fmt.Errorf("PasskeyUseCase - BeginLogin - uc.signSession: %w", err)
	}

	return options, token, nil
}

// FinishLogin verifies the browser's assertion (PublicKeyCredential.toJSON) against the ceremony in
// token, burns the ceremony's challenge, records the credential's use, and returns an auth token for
// the passkey's owner.
//
// The burn comes after the assertion verifies and before anything is written or issued: only a
// signature from a registered passkey can consume a challenge, so a request carrying a captured token
// and a forged body cannot burn a ceremony its owner is still completing, and a replay of a verified
// assertion still fails, because the insert that burns the challenge is the check.
func (uc *UseCase) FinishLogin(ctx context.Context, token string, body []byte) (string, error) {
	session, err := uc.verifySession(token)
	if err != nil {
		return "", fmt.Errorf("PasskeyUseCase - FinishLogin - uc.verifySession: %w", err)
	}

	parsed, err := protocol.ParseCredentialRequestResponseBody(bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("PasskeyUseCase - FinishLogin - protocol.ParseCredentialRequestResponseBody: %w: %w",
			entity.ErrPasskeyCeremonyInvalid, err)
	}

	owner, credential, err := uc.validateLogin(ctx, &session, parsed)
	if err != nil {
		return "", fmt.Errorf("PasskeyUseCase - FinishLogin - uc.validateLogin: %w", err)
	}

	err = uc.burnChallenge(ctx, &session)
	if err != nil {
		return "", fmt.Errorf("PasskeyUseCase - FinishLogin - uc.burnChallenge: %w", err)
	}

	err = uc.recordUsage(ctx, credential)
	if err != nil {
		return "", fmt.Errorf("PasskeyUseCase - FinishLogin - uc.recordUsage: %w", err)
	}

	authToken, err := uc.tokens.GenerateToken(owner.id)
	if err != nil {
		return "", fmt.Errorf("PasskeyUseCase - FinishLogin - uc.tokens.GenerateToken: %w", err)
	}

	return authToken, nil
}

// validateLogin runs the WebAuthn assertion checks against the account the user handle names, and
// rejects a credential whose non-zero sign count did not advance (a possible clone). A lookup that
// fails for a reason other than an unknown account is returned as is, not as a bad ceremony.
func (uc *UseCase) validateLogin(
	ctx context.Context, session *webauthn.SessionData, parsed *protocol.ParsedCredentialAssertionData,
) (*account, *webauthn.Credential, error) {
	var lookupErr error

	handler := func(_, handle []byte) (webauthn.User, error) {
		owner, err := uc.lookupAccount(ctx, handle)
		if err != nil {
			lookupErr = err

			return nil, err
		}

		return owner, nil
	}

	user, credential, err := uc.webauthn.ValidatePasskeyLogin(handler, *session, parsed)

	if lookupErr != nil && !errors.Is(lookupErr, entity.ErrPasskeyCeremonyInvalid) {
		return nil, nil, lookupErr
	}

	if err != nil {
		return nil, nil, fmt.Errorf("uc.webauthn.ValidatePasskeyLogin: %w: %w", entity.ErrPasskeyCeremonyInvalid, err)
	}

	if credential.Authenticator.CloneWarning {
		return nil, nil, fmt.Errorf("sign count did not advance: %w", entity.ErrPasskeyCeremonyInvalid)
	}

	owner, ok := user.(*account)
	if !ok {
		return nil, nil, fmt.Errorf("unexpected user %T: %w", user, entity.ErrPasskeyCeremonyInvalid)
	}

	return owner, credential, nil
}

// lookupAccount is the discoverable login handler: the user handle is the owner's UUID bytes. A
// handle that names no user is entity.ErrPasskeyCeremonyInvalid.
func (uc *UseCase) lookupAccount(ctx context.Context, handle []byte) (*account, error) {
	id, err := uuid.FromBytes(handle)
	if err != nil {
		return nil, fmt.Errorf("uuid.FromBytes: %w: %w", entity.ErrPasskeyCeremonyInvalid, err)
	}

	user, err := uc.users.GetByID(ctx, id.String())
	if err != nil {
		if errors.Is(err, entity.ErrUserNotFound) {
			return nil, fmt.Errorf("uc.users.GetByID: %w: %w", entity.ErrPasskeyCeremonyInvalid, err)
		}

		return nil, fmt.Errorf("uc.users.GetByID: %w", err)
	}

	stored, err := uc.passkeys.ListByUser(ctx, user.ID)
	if err != nil {
		return nil, fmt.Errorf("uc.passkeys.ListByUser: %w", err)
	}

	return newAccount(&user, stored)
}

// burnChallenge marks the session's challenge used until the ceremony expires, and forgets the
// challenges of ceremonies that already have. A challenge burned before is entity.ErrPasskeyChallengeUsed.
func (uc *UseCase) burnChallenge(ctx context.Context, session *webauthn.SessionData) error {
	err := uc.passkeys.DeleteExpiredChallenges(ctx)
	if err != nil {
		return fmt.Errorf("uc.passkeys.DeleteExpiredChallenges: %w", err)
	}

	expiresAt := session.Expires
	if expiresAt.IsZero() {
		expiresAt = time.Now().Add(uc.timeout)
	}

	hash := sha256.Sum256([]byte(session.Challenge))

	err = uc.passkeys.BurnChallenge(ctx, hash[:], expiresAt.UTC())
	if err != nil {
		return fmt.Errorf("uc.passkeys.BurnChallenge: %w", err)
	}

	return nil
}

// recordUsage stores the credential as validation left it (sign count, flags) and stamps last use.
func (uc *UseCase) recordUsage(ctx context.Context, credential *webauthn.Credential) error {
	record, err := json.Marshal(credential)
	if err != nil {
		return fmt.Errorf("json.Marshal credential: %w", err)
	}

	err = uc.passkeys.UpdateUsage(ctx, credential.ID, record, time.Now().UTC())
	if err != nil {
		return fmt.Errorf("uc.passkeys.UpdateUsage: %w", err)
	}

	return nil
}

// signSession seals the ceremony state into a token the client hands back on finish.
func (uc *UseCase) signSession(session *webauthn.SessionData) (string, error) {
	payload, err := json.Marshal(session)
	if err != nil {
		return "", fmt.Errorf("json.Marshal: %w", err)
	}

	token, err := uc.ceremony.Sign(payload)
	if err != nil {
		return "", fmt.Errorf("uc.ceremony.Sign: %w", err)
	}

	return token, nil
}

// verifySession opens a token from signSession. A token that is forged, expired, or not a ceremony
// token is entity.ErrPasskeyCeremonyInvalid.
func (uc *UseCase) verifySession(token string) (webauthn.SessionData, error) {
	payload, err := uc.ceremony.Verify(token)
	if err != nil {
		return webauthn.SessionData{}, fmt.Errorf("uc.ceremony.Verify: %w: %w", entity.ErrPasskeyCeremonyInvalid, err)
	}

	var session webauthn.SessionData

	err = json.Unmarshal(payload, &session)
	if err != nil {
		return webauthn.SessionData{}, fmt.Errorf("json.Unmarshal: %w: %w", entity.ErrPasskeyCeremonyInvalid, err)
	}

	return session, nil
}

// userHandle is the WebAuthn user handle for a user: their UUID's 16 bytes. An ID that is not a
// UUID names no account a ceremony could belong to.
func userHandle(userID string) ([]byte, error) {
	id, err := uuid.Parse(userID)
	if err != nil {
		return nil, fmt.Errorf("uuid.Parse: %w: %w", entity.ErrPasskeyCeremonyInvalid, err)
	}

	return id[:], nil
}

// account adapts a user and their stored passkeys to webauthn.User.
type account struct {
	id          string
	handle      []byte
	name        string
	displayName string
	credentials []webauthn.Credential
}

var _ webauthn.User = (*account)(nil)

func newAccount(user *entity.User, stored []entity.Passkey) (*account, error) {
	handle, err := userHandle(user.ID)
	if err != nil {
		return nil, err
	}

	credentials := make([]webauthn.Credential, len(stored))
	for i := range stored {
		err = json.Unmarshal(stored[i].Credential, &credentials[i])
		if err != nil {
			return nil, fmt.Errorf("json.Unmarshal passkey %s: %w", stored[i].ID, err)
		}
	}

	return &account{
		id:          user.ID,
		handle:      handle,
		name:        user.Email,
		displayName: user.Username,
		credentials: credentials,
	}, nil
}

func (a *account) WebAuthnID() []byte                         { return a.handle }
func (a *account) WebAuthnName() string                       { return a.name }
func (a *account) WebAuthnDisplayName() string                { return a.displayName }
func (a *account) WebAuthnCredentials() []webauthn.Credential { return a.credentials }
