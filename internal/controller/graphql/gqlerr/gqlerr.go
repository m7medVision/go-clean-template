// Package gqlerr builds the errors GraphQL resolvers return, each with a code in extensions.code.
package gqlerr

import (
	"context"
	"log/slog"

	"github.com/m7medvision/go-clean-template/internal/entity"
	"github.com/vektah/gqlparser/v2/gqlerror"
)

const (
	_codeUnauthenticated = "UNAUTHENTICATED"
	_codeBadUserInput    = "BAD_USER_INPUT"
	_codeNotFound        = "NOT_FOUND"
	_codeForbidden       = "FORBIDDEN"
	_codeUnavailable     = "UNAVAILABLE"
	_codeConflict        = "CONFLICT"
	_codeNotImplemented  = "NOT_IMPLEMENTED"
	_codeInternal        = "INTERNAL"
)

// Unauthenticated is the error for a request without a valid bearer token.
func Unauthenticated() error {
	return newError(_codeUnauthenticated, "unauthorized")
}

// InvalidInput is the error for arguments that fail a resolver's checks.
func InvalidInput() error {
	return newError(_codeBadUserInput, "invalid input")
}

// Internal is the error for a failure whose details the caller must not see.
func Internal() error {
	return newError(_codeInternal, "internal server error")
}

// Map turns a failed command or query into the GraphQL error for the kind of its entity.Error,
// logged under op as its kind says.
func Map(ctx context.Context, l *slog.Logger, err error, op string) error {
	kind := entity.KindOf(err)
	if level, ok := kind.LogLevel(); ok {
		l.Log(ctx, level, "graphql - "+op, "error", err)
	}

	return newError(code(kind), entity.Message(err))
}

func code(k entity.Kind) string {
	return [...]string{
		entity.KindInternal:        _codeInternal,
		entity.KindInvalid:         _codeBadUserInput,
		entity.KindUnauthenticated: _codeUnauthenticated,
		entity.KindForbidden:       _codeForbidden,
		entity.KindNotFound:        _codeNotFound,
		entity.KindExists:          _codeConflict,
		entity.KindConflict:        _codeConflict,
		entity.KindUnavailable:     _codeUnavailable,
		entity.KindUpstream:        _codeUnavailable,
		entity.KindNotImplemented:  _codeNotImplemented,
	}[k]
}

func newError(code, message string) error {
	return &gqlerror.Error{Message: message, Extensions: map[string]any{"code": code}}
}
