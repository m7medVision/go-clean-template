package rpcerr_test

import (
	"errors"
	"fmt"
	"testing"

	"github.com/m7medvision/go-clean-template/internal/controller/grpc/v1/rpcerr"
	"github.com/m7medvision/go-clean-template/internal/entity"
	"github.com/stretchr/testify/assert"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestSharedMapsTheFailuresAnyCommandCanProduce(t *testing.T) {
	t.Parallel()

	assert.Equal(t, codes.Unavailable, status.Code(rpcerr.Shared(fmt.Errorf("w: %w", entity.ErrReadModelBehind))))
	assert.Equal(t, codes.Aborted, status.Code(rpcerr.Shared(fmt.Errorf("w: %w", entity.ErrConcurrencyConflict))))
	assert.Equal(t, codes.Unimplemented, status.Code(rpcerr.Shared(fmt.Errorf("w: %w", entity.ErrNotImplemented))))
}

func TestSharedLeavesOtherErrorsToTheRPC(t *testing.T) {
	t.Parallel()

	assert.NoError(t, rpcerr.Shared(errors.New("anything else"))) //nolint:err113 // a one-off error is the point
	assert.NoError(t, rpcerr.Shared(entity.ErrTaskNotFound))
}
