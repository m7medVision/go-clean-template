package entity

import (
	"errors"
	"fmt"
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
	KindRateLimited
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
	case KindInvalid, KindUnauthenticated, KindForbidden, KindNotFound, KindExists, KindRateLimited:
		return 0, false
	case KindConflict, KindUnavailable, KindNotImplemented:
		return slog.LevelWarn, true
	case KindInternal, KindUpstream:
	}

	return slog.LevelError, true
}

var (
	// ErrAdminRequired is a request only a caller holding RoleAdmin may make.
	ErrAdminRequired = newError(KindForbidden, "admin role required")
	// ErrInvalidPage is a list asked for with a negative offset or a limit outside 0–MaxPageLimit.
	ErrInvalidPage = newError(KindInvalid,
		fmt.Sprintf("limit must be 0–%d (0 means %d) and offset at least 0", MaxPageLimit, DefaultPageLimit))
	// ErrConcurrencyConflict is an append whose stream moved on since the aggregate was loaded:
	// another write got there first. The command bus retries the command before giving up.
	ErrConcurrencyConflict = newError(KindConflict, "the record changed while this request ran: try again")
	// ErrReadModelBehind is a write that committed but whose projections had not caught up within
	// the consistency timeout, so a read straight after it could still miss it.
	ErrReadModelBehind = newError(KindUnavailable, "saved, but not yet readable: read it again shortly")
	// ErrNotImplemented is a command or query whose handler is still the stub the generator wrote
	// (tools/gen).
	ErrNotImplemented = newError(KindNotImplemented, "not implemented")

	// ErrStreamNotFound is a load of a stream that has no events. Command handlers turn it into
	// their domain's not-found error.
	ErrStreamNotFound = errors.New("stream not found")
	// ErrClaimTaken is a claim another owner holds, when the claim names no Taken error of its own.
	ErrClaimTaken = errors.New("claim taken")
	// ErrUnhandledEvent is an event handed to a subscriber whose Handles lists it but whose Handle
	// has no case for it: a bug the subscriber stops on rather than skipping the event.
	ErrUnhandledEvent = errors.New("event its subscriber has no case for")
	// ErrClaimNotFound is a lookup of a claim nobody holds.
	ErrClaimNotFound = errors.New("claim not found")
)
