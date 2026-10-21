package entity

import "errors"

var (
	ErrUserNotFound       = errors.New("user not found")
	ErrUserAlreadyExists  = errors.New("user already exists")
	ErrInvalidCredentials = errors.New("invalid credentials")
	ErrTaskNotFound       = errors.New("task not found")
	ErrTaskForbidden      = errors.New("task does not belong to user")
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
	// ErrAdminRequired is a request only a caller holding RoleAdmin may make.
	ErrAdminRequired       = errors.New("admin role required")
	ErrInvalidTransition   = errors.New("invalid status transition")
	ErrUnknownEmail        = errors.New("unknown email template")
	ErrInvalidConversion   = errors.New("invalid currency conversion")
	ErrUnsupportedCurrency = errors.New("unsupported currency")
	ErrPasskeyNotFound     = errors.New("passkey not found")
	ErrPasskeyExists       = errors.New("passkey already registered")
	// ErrPasskeyCeremonyInvalid is a WebAuthn ceremony that cannot complete: a bad, expired or
	// foreign ceremony token, or a credential response that does not verify against it.
	ErrPasskeyCeremonyInvalid = errors.New("invalid passkey ceremony")
	// ErrPasskeyChallengeUsed is a login ceremony whose challenge has already completed once: a replay.
	ErrPasskeyChallengeUsed = errors.New("passkey challenge already used")
	// ErrPasskeyReauthRequired is a request to add a passkey from a sign-in older than the allowed
	// age: the caller must sign in again first.
	ErrPasskeyReauthRequired = errors.New("passkey registration requires a recent sign-in")
)
