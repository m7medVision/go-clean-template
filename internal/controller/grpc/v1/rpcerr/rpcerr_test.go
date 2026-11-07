package rpcerr_test

import (
	"errors"
	"fmt"
	"log/slog"
	"testing"

	"github.com/m7medvision/go-clean-template/internal/controller/grpc/v1/rpcerr"
	"github.com/m7medvision/go-clean-template/internal/entity"
	"github.com/stretchr/testify/assert"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestStatusAnswersEachKindWithItsCodeAndMessage(t *testing.T) {
	t.Parallel()

	l := slog.New(slog.DiscardHandler)

	for _, tc := range []struct {
		err  error
		code codes.Code
		msg  string
	}{
		{entity.ErrInvalidTransition, codes.InvalidArgument, "invalid status transition"},
		{entity.ErrInvalidCredentials, codes.Unauthenticated, "invalid credentials"},
		{entity.ErrTaskForbidden, codes.PermissionDenied, "forbidden"},
		{entity.ErrTaskNotFound, codes.NotFound, "task not found"},
		{entity.ErrUserAlreadyExists, codes.AlreadyExists, "user already exists"},
		{entity.ErrConcurrencyConflict, codes.Aborted, entity.ErrConcurrencyConflict.Error()},
		{entity.ErrReadModelBehind, codes.Unavailable, entity.ErrReadModelBehind.Error()},
		{entity.ErrOIDCUnavailable, codes.Unavailable, "sso provider unavailable"},
		{entity.ErrNotImplemented, codes.Unimplemented, "not implemented"},
		{errors.New("db down"), codes.Internal, "internal server error"}, //nolint:err113 // a one-off error is the point
	} {
		st, ok := status.FromError(rpcerr.Status(t.Context(), l, fmt.Errorf("op: %w", tc.err), "op"))
		assert.True(t, ok, tc.err)
		assert.Equal(t, tc.code, st.Code(), tc.err)
		assert.Equal(t, tc.msg, st.Message(), tc.err)
	}
}
