package entity

import "errors"

var (
	ErrUserNotFound       = errors.New("user not found")
	ErrUserAlreadyExists  = errors.New("user already exists")
	ErrInvalidCredentials = errors.New("invalid credentials")
	ErrTaskNotFound       = errors.New("task not found")
	ErrTaskForbidden      = errors.New("task does not belong to user")
	// ErrAdminRequired is a request only a caller holding RoleAdmin may make.
	ErrAdminRequired = errors.New("admin role required")
	// ErrRoleRequired is a request only a caller holding some other role may make.
	ErrRoleRequired        = errors.New("role required")
	ErrInvalidTransition   = errors.New("invalid status transition")
	ErrUnknownEmail        = errors.New("unknown email template")
	ErrInvalidConversion   = errors.New("invalid currency conversion")
	ErrConversionNotFound  = errors.New("conversion not found")
	ErrUnsupportedCurrency = errors.New("unsupported currency")
	// ErrOIDCDisabled is an SSO request to an app with no OpenID Connect provider configured.
	ErrOIDCDisabled = errors.New("sso sign-in is not configured")
	// ErrOIDCUnavailable is an SSO sign-in while the OpenID Connect provider cannot be reached.
	ErrOIDCUnavailable = errors.New("sso provider unavailable")
	// ErrOIDCCeremonyInvalid is an SSO sign-in that cannot complete: a bad, expired or replayed
	// state token, code or ID token, or an ID token without an email.
	ErrOIDCCeremonyInvalid = errors.New("invalid sso sign-in")
	// ErrAccountConflict is an SSO sign-in whose email or username already belongs to another account.
	// Accounts are never linked by email, so the sign-in is refused.
	ErrAccountConflict = errors.New("an account with this email or username already exists")
	ErrPasskeyNotFound = errors.New("passkey not found")
	ErrPasskeyExists   = errors.New("passkey already registered")
	// ErrPasskeyCeremonyInvalid is a WebAuthn ceremony that cannot complete: a bad, expired or
	// foreign ceremony token, or a credential response that does not verify against it.
	ErrPasskeyCeremonyInvalid = errors.New("invalid passkey ceremony")
	// ErrPasskeyChallengeUsed is a login ceremony whose challenge has already completed once: a replay.
	ErrPasskeyChallengeUsed = errors.New("passkey challenge already used")
	// ErrPasskeyReauthRequired is a request to add a passkey from a sign-in older than the allowed
	// age: the caller must sign in again first.
	ErrPasskeyReauthRequired = errors.New("passkey registration requires a recent sign-in")
	// ErrStreamNotFound is a load of a stream that has no events. Command handlers turn it into
	// their domain's not-found error.
	ErrStreamNotFound = errors.New("stream not found")
	// ErrClaimNotFound is a lookup of a claim nobody holds.
	ErrClaimNotFound = errors.New("claim not found")
	// ErrConcurrencyConflict is an append whose stream moved on since the aggregate was loaded:
	// another write got there first. The command bus retries the command before giving up.
	ErrConcurrencyConflict = errors.New("concurrent write to the same record")
	// ErrReadModelBehind is a write that committed but whose projections had not caught up within
	// the consistency timeout, so a read straight after it could still miss it.
	ErrReadModelBehind = errors.New("write saved but not yet readable")
	// ErrNotImplemented is a command or query whose handler is still the stub the generator wrote
	// (tools/gen). Like the two above, any message can produce it, so each transport maps it once.
	ErrNotImplemented = errors.New("not implemented")
)
