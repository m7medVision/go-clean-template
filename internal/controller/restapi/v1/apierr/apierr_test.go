package apierr_test

import (
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"testing"
	"time"

	"github.com/danielgtaylor/huma/v2"
	"github.com/m7medvision/go-clean-template/internal/controller/restapi/v1/apierr"
	"github.com/m7medvision/go-clean-template/internal/entity"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestProblemAnswersEachKindWithItsStatusAndMessage(t *testing.T) {
	t.Parallel()

	l := slog.New(slog.DiscardHandler)

	for _, tc := range []struct {
		err    error
		status int
		detail string
	}{
		{entity.ErrInvalidTransition, http.StatusBadRequest, "invalid status transition"},
		{entity.ErrInvalidCredentials, http.StatusUnauthorized, "invalid credentials"},
		{entity.ErrTaskForbidden, http.StatusForbidden, "forbidden"},
		{entity.ErrTaskNotFound, http.StatusNotFound, "task not found"},
		{entity.ErrUserAlreadyExists, http.StatusConflict, "user already exists"},
		{entity.ErrConcurrencyConflict, http.StatusConflict, entity.ErrConcurrencyConflict.Error()},
		{entity.ErrReadModelBehind, http.StatusServiceUnavailable, entity.ErrReadModelBehind.Error()},
		{entity.ErrOIDCUnavailable, http.StatusBadGateway, "sso provider unavailable"},
		{entity.ErrNotImplemented, http.StatusNotImplemented, "not implemented"},
		{entity.ErrTooManyAttempts, http.StatusTooManyRequests, "too many attempts"},
		{errors.New("db down"), http.StatusInternalServerError, "internal server error"}, //nolint:err113 // a one-off error is the point
	} {
		var se huma.StatusError

		require.ErrorAs(t, apierr.Problem(t.Context(), l, fmt.Errorf("op: %w", tc.err), "op"), &se)
		assert.Equal(t, tc.status, se.GetStatus(), tc.err)

		var model *huma.ErrorModel

		require.ErrorAs(t, se, &model)
		assert.Equal(t, tc.detail, model.Detail, tc.err)
	}
}

func TestProblemTellsARateLimitedCallerWhenToRetry(t *testing.T) {
	t.Parallel()

	l := slog.New(slog.DiscardHandler)

	for _, tc := range []struct {
		name       string
		err        error
		retryAfter []string
	}{
		{"a wait in whole seconds", entity.TooManyAttempts(3 * time.Second), []string{"3"}},
		{"a part second rounded up", entity.TooManyAttempts(1500 * time.Millisecond), []string{"2"}},
		{"never below one second", entity.TooManyAttempts(100 * time.Millisecond), []string{"1"}},
		{"a wait already over", entity.TooManyAttempts(0), []string{"1"}},
		{"no header without a wait", entity.ErrTooManyAttempts, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			problem := apierr.Problem(t.Context(), l, fmt.Errorf("op: %w", tc.err), "op")

			var se huma.StatusError

			require.ErrorAs(t, problem, &se)
			assert.Equal(t, http.StatusTooManyRequests, se.GetStatus())

			he, ok := errors.AsType[huma.HeadersError](problem)
			if tc.retryAfter == nil {
				assert.False(t, ok)

				return
			}

			require.True(t, ok)
			assert.Equal(t, http.Header{"Retry-After": tc.retryAfter}, he.GetHeaders())
		})
	}
}

func TestCommandErrorsAddWhatAnyCommandCanAnswer(t *testing.T) {
	t.Parallel()

	assert.Equal(t,
		[]int{http.StatusUnauthorized, http.StatusNotFound, http.StatusConflict, http.StatusInternalServerError, http.StatusServiceUnavailable},
		apierr.CommandErrors(http.StatusNotFound, http.StatusUnauthorized, http.StatusConflict))
	assert.Equal(t, []int{http.StatusUnauthorized, http.StatusInternalServerError}, apierr.QueryErrors(http.StatusUnauthorized))
}
