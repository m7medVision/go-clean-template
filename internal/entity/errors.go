package entity

import (
	"errors"
	"log/slog"
)

// Kind is what a failure means to the caller, whatever the transport: each transport turns a kind
// into its own status once.
type Kind int

// The kinds. An error that is not an *Error is KindInternal: its details are logged, never shown.
const (
	KindInternal Kind = iota
	KindInvalid
	KindUnauthenticated
	KindForbidden
	KindNotFound
	KindExists
	KindConflict
	KindUnavailable
	KindUpstream
	KindNotImplemented
)

// Error is a failure the caller may be told about: its message is safe to show.
type Error struct {
	kind Kind
	msg  string
}

func newError(kind Kind, msg string) *Error {
	return &Error{kind: kind, msg: msg}
}

func (e *Error) Error() string { return e.msg }

// Kind is what the failure means to the caller.
func (e *Error) Kind() Kind { return e.kind }

// KindOf is the kind of the first *Error in err's chain, or KindInternal.
func KindOf(err error) Kind {
	if e, ok := errors.AsType[*Error](err); ok {
		return e.kind
	}

	return KindInternal
}

// Message is what the caller may be told about err: the message of the first *Error in its chain,
// or a generic one for an internal failure.
func Message(err error) string {
	var e *Error
	if errors.As(err, &e) && e.kind != KindInternal {
		return e.msg
	}

	return "internal server error"
}

// LogLevel is how a failure of this kind is logged where it is answered: the caller's own mistakes
// not at all, conditions that pass (a lost race, a read model still behind, a stub) as warnings,
// and the app's own faults, its dependencies' included, as errors.
func (k Kind) LogLevel() (slog.Level, bool) {
	switch k {
	case KindInvalid, KindUnauthenticated, KindForbidden, KindNotFound, KindExists:
		return 0, false
	case KindConflict, KindUnavailable, KindNotImplemented:
		return slog.LevelWarn, true
	case KindInternal, KindUpstream:
	}

	return slog.LevelError, true
}

var (
	ErrUserNotFound           = newError(KindNotFound, "user not found")
	ErrUserAlreadyExists      = newError(KindExists, "user already exists")
	ErrInvalidCredentials     = newError(KindUnauthenticated, "invalid credentials")
	ErrInvalidUsername        = newError(KindInvalid, "username must be 3–255 characters")
	ErrInvalidEmail           = newError(KindInvalid, "email must be an address such as name@example.com")
	ErrWeakPassword           = newError(KindInvalid, "password must be at least 6 characters")
	ErrTaskNotFound           = newError(KindNotFound, "task not found")
	ErrTaskForbidden          = newError(KindForbidden, "forbidden")
	ErrInvalidTaskTitle       = newError(KindInvalid, "title must be 1–255 characters")
	ErrInvalidTaskDescription = newError(KindInvalid, "description must be at most 1000 characters")
	ErrInvalidTaskStatus      = newError(KindInvalid, "invalid task status")
	// ErrAdminRequired is a request only a caller holding RoleAdmin may make.
	ErrAdminRequired     = newError(KindForbidden, "admin role required")
	ErrInvalidTransition = newError(KindInvalid, "invalid status transition")
	// ErrInvalidPage is a list asked for with a negative offset or a limit outside 0–MaxPageLimit.
	ErrInvalidPage         = newError(KindInvalid, "limit must be 1–100 and offset at least 0")
	ErrInvalidConversion   = newError(KindInvalid, "invalid conversion")
	ErrConversionNotFound  = newError(KindNotFound, "conversion not found")
	ErrUnsupportedCurrency = newError(KindInvalid, "unsupported currency")
	ErrInvalidAmount       = newError(KindInvalid,
		"amount must be positive, with at most 15 digits before the point and 8 after it")
	ErrInvalidPasskeyName = newError(KindInvalid, "passkey name must be at most 64 characters")
	// ErrOIDCDisabled is an SSO request to an app with no OpenID Connect provider configured.
	ErrOIDCDisabled = newError(KindNotFound, "sso sign-in is not configured")
	// ErrOIDCUnavailable is an SSO sign-in while the OpenID Connect provider cannot be reached.
	ErrOIDCUnavailable = newError(KindUpstream, "sso provider unavailable")
	// ErrOIDCCeremonyInvalid is an SSO sign-in that cannot complete: a bad, expired or replayed
	// state token, code or ID token, or an ID token without an email.
	ErrOIDCCeremonyInvalid = newError(KindUnauthenticated, "invalid sso sign-in")
	// ErrAccountConflict is an SSO sign-in whose email or username already belongs to another account.
	// Accounts are never linked by email, so the sign-in is refused.
	ErrAccountConflict = newError(KindExists,
		"an account with this email or username already exists; sign in with it instead")
	ErrPasskeyNotFound = newError(KindNotFound, "passkey not found")
	ErrPasskeyExists   = newError(KindExists, "passkey already registered")
	// ErrPasskeyCeremonyInvalid is a WebAuthn registration that cannot complete: a bad, expired or
	// foreign ceremony token, or a credential response that does not verify against it.
	ErrPasskeyCeremonyInvalid = newError(KindInvalid, "invalid passkey ceremony")
	// ErrPasskeyLoginInvalid is a passkey sign-in that cannot complete, for the same reasons.
	ErrPasskeyLoginInvalid = newError(KindUnauthenticated, "invalid passkey login")
	// ErrPasskeyChallengeUsed is a login ceremony whose challenge has already completed once: a replay.
	ErrPasskeyChallengeUsed = newError(KindConflict, "passkey login already completed")
	// ErrPasskeyReauthRequired is a request to add a passkey from a sign-in older than the allowed
	// age: the caller must sign in again first. Clients match on its message, so it must not change.
	ErrPasskeyReauthRequired = newError(KindUnauthenticated, "reauthentication required")
	// ErrConcurrencyConflict is an append whose stream moved on since the aggregate was loaded:
	// another write got there first. The command bus retries the command before giving up.
	ErrConcurrencyConflict = newError(KindConflict, "the record changed while this request ran: try again")
	// ErrReadModelBehind is a write that committed but whose projections had not caught up within
	// the consistency timeout, so a read straight after it could still miss it.
	ErrReadModelBehind = newError(KindUnavailable, "saved, but not yet readable: read it again shortly")
	// ErrNotImplemented is a command or query whose handler is still the stub the generator wrote
	// (tools/gen).
	ErrNotImplemented = newError(KindNotImplemented, "not implemented")

	ErrUnknownEmail = errors.New("unknown email template")
	// ErrStreamNotFound is a load of a stream that has no events. Command handlers turn it into
	// their domain's not-found error.
	ErrStreamNotFound = errors.New("stream not found")
	// ErrClaimNotFound is a lookup of a claim nobody holds.
	ErrClaimNotFound = errors.New("claim not found")
)
