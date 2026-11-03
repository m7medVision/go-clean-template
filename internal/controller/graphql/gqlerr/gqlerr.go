// Package gqlerr builds the errors GraphQL resolvers return, each with a code in extensions.code.
package gqlerr

import (
	"errors"

	"github.com/m7medvision/go-clean-template/internal/entity"
	"github.com/m7medvision/go-clean-template/pkg/logger"
	"github.com/vektah/gqlparser/v2/gqlerror"
)

const (
	_codeUnauthenticated = "UNAUTHENTICATED"
	_codeBadUserInput    = "BAD_USER_INPUT"
	_codeNotFound        = "NOT_FOUND"
	_codeForbidden       = "FORBIDDEN"
	_codeInternal        = "INTERNAL"
)

type mapping struct {
	sentinel error
	code     string
	message  string
}

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

// Map turns a use-case error into the GraphQL error for its entity.Err* sentinel. Any other error
// is logged under op and reported as Internal.
func Map(l logger.Interface, err error, op string) error {
	for _, m := range []mapping{
		{entity.ErrTaskNotFound, _codeNotFound, "task not found"},
		{entity.ErrTaskForbidden, _codeForbidden, "forbidden"},
		{entity.ErrAdminRequired, _codeForbidden, "admin role required"},
		{entity.ErrInvalidTransition, _codeBadUserInput, "invalid status transition"},
	} {
		if errors.Is(err, m.sentinel) {
			return newError(m.code, m.message)
		}
	}

	l.Error(err, "graphql - "+op)

	return Internal()
}

func newError(code, message string) error {
	return &gqlerror.Error{Message: message, Extensions: map[string]any{"code": code}}
}
