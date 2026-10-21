package entity

import "errors"

var (
	ErrUserNotFound        = errors.New("user not found")
	ErrUserAlreadyExists   = errors.New("user already exists")
	ErrInvalidCredentials  = errors.New("invalid credentials")
	ErrTaskNotFound        = errors.New("task not found")
	ErrTaskForbidden       = errors.New("task does not belong to user")
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
