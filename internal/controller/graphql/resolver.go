package graphql

import (
	"context"
	"errors"
	"unicode/utf8"

	"github.com/evrone/go-clean-template/internal/entity"
	"github.com/evrone/go-clean-template/internal/usecase"
	"github.com/evrone/go-clean-template/pkg/logger"
	"github.com/vektah/gqlparser/v2/gqlerror"
)

const (
	_codeUnauthenticated = "UNAUTHENTICATED"
	_codeBadUserInput    = "BAD_USER_INPUT"
	_codeNotFound        = "NOT_FOUND"
	_codeForbidden       = "FORBIDDEN"
	_codeInternal        = "INTERNAL"

	_msgUnauthorized = "unauthorized"
	_msgInvalidInput = "invalid input"
	_msgInternal     = "internal server error"

	_maxTitleLen       = 255
	_maxDescriptionLen = 1000
	_minLimit          = 1
)

// Resolver -.
type Resolver struct {
	task usecase.Task
	l    logger.Interface
}

type errorMapping struct {
	sentinel error
	code     string
	message  string
}

func newError(code, message string) *gqlerror.Error {
	return &gqlerror.Error{Message: message, Extensions: map[string]any{"code": code}}
}

func caller(ctx context.Context) (string, error) {
	userID, ok := UserID(ctx)
	if !ok {
		return "", newError(_codeUnauthenticated, _msgUnauthorized)
	}

	return userID, nil
}

func validTaskInput(title string, description *string) bool {
	titleLen := utf8.RuneCountInString(title)

	return titleLen > 0 && titleLen <= _maxTitleLen &&
		(description == nil || utf8.RuneCountInString(*description) <= _maxDescriptionLen)
}

func (r *Resolver) mapError(err error, op string) error {
	for _, m := range []errorMapping{
		{entity.ErrTaskNotFound, _codeNotFound, "task not found"},
		{entity.ErrTaskForbidden, _codeForbidden, "forbidden"},
		{entity.ErrInvalidTransition, _codeBadUserInput, "invalid status transition"},
	} {
		if errors.Is(err, m.sentinel) {
			return newError(m.code, m.message)
		}
	}

	r.l.Error(err, "graphql - "+op)

	return newError(_codeInternal, _msgInternal)
}

func deref(s *string) string {
	if s == nil {
		return ""
	}

	return *s
}
