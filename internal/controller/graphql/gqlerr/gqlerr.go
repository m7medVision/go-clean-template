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
	_codeRateLimited     = "RATE_LIMITED"
	_codeInternal        = "INTERNAL"
)

// Unauthenticated is the error for a request without a valid bearer token.
func Unauthenticated() error {
	return newError(_codeUnauthenticated, "unauthorized")
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

// code is the extensions.code of a kind. A switch, so that exhaustive fails the lint on a kind it
// does not name.
func code(k entity.Kind) string {
	switch k {
	case entity.KindInvalid:
		return _codeBadUserInput
	case entity.KindUnauthenticated:
		return _codeUnauthenticated
	case entity.KindForbidden:
		return _codeForbidden
	case entity.KindNotFound:
		return _codeNotFound
	case entity.KindExists, entity.KindConflict:
		return _codeConflict
	case entity.KindUnavailable, entity.KindUpstream:
		return _codeUnavailable
	case entity.KindNotImplemented:
		return _codeNotImplemented
	case entity.KindRateLimited:
		return _codeRateLimited
	case entity.KindInternal:
	}

	return _codeInternal
}

func newError(code, message string) error {
	return &gqlerror.Error{Message: message, Extensions: map[string]any{"code": code}}
}
