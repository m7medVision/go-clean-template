package apierr_test

import (
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"testing"

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

func TestCommandErrorsAddWhatAnyCommandCanAnswer(t *testing.T) {
	t.Parallel()

	assert.Equal(t,
		[]int{http.StatusUnauthorized, http.StatusNotFound, http.StatusConflict, http.StatusInternalServerError, http.StatusServiceUnavailable},
		apierr.CommandErrors(http.StatusNotFound, http.StatusUnauthorized, http.StatusConflict))
	assert.Equal(t, []int{http.StatusUnauthorized, http.StatusInternalServerError}, apierr.QueryErrors(http.StatusUnauthorized))
}
