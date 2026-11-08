package gqlerr_test

import (
	"errors"
	"fmt"
	"log/slog"
	"testing"
	"time"

	"github.com/m7medvision/go-clean-template/internal/controller/graphql/gqlerr"
	"github.com/m7medvision/go-clean-template/internal/entity"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/vektah/gqlparser/v2/gqlerror"
)

func TestMapAnswersEachKindWithItsCodeAndMessage(t *testing.T) {
	t.Parallel()

	l := slog.New(slog.DiscardHandler)

	for _, tc := range []struct {
		err  error
		code string
		msg  string
	}{
		{entity.ErrInvalidTransition, "BAD_USER_INPUT", "invalid status transition"},
		{entity.ErrInvalidCredentials, "UNAUTHENTICATED", "invalid credentials"},
		{entity.ErrTaskForbidden, "FORBIDDEN", "forbidden"},
		{entity.ErrTaskNotFound, "NOT_FOUND", "task not found"},
		{entity.ErrUserAlreadyExists, "CONFLICT", "user already exists"},
		{entity.ErrConcurrencyConflict, "CONFLICT", entity.ErrConcurrencyConflict.Error()},
		{entity.ErrReadModelBehind, "UNAVAILABLE", entity.ErrReadModelBehind.Error()},
		{entity.ErrOIDCUnavailable, "UNAVAILABLE", "sso provider unavailable"},
		{entity.ErrNotImplemented, "NOT_IMPLEMENTED", "not implemented"},
		{entity.TooManyAttempts(time.Minute), "RATE_LIMITED", "too many attempts"},
		{errors.New("db down"), "INTERNAL", "internal server error"}, //nolint:err113 // a one-off error is the point
	} {
		var gqlErr *gqlerror.Error

		require.ErrorAs(t, gqlerr.Map(t.Context(), l, fmt.Errorf("op: %w", tc.err), "op"), &gqlErr)
		assert.Equal(t, tc.code, gqlErr.Extensions["code"], tc.err)
		assert.Equal(t, tc.msg, gqlErr.Message, tc.err)
	}
}
