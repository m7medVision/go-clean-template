package apierr_test

import (
	"errors"
	"fmt"
	"net/http"
	"testing"

	"github.com/danielgtaylor/huma/v2"
	"github.com/m7medvision/go-clean-template/internal/controller/restapi/v1/apierr"
	"github.com/m7medvision/go-clean-template/internal/entity"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func status(t *testing.T, err error) int {
	t.Helper()

	var se huma.StatusError
	require.ErrorAs(t, err, &se)

	return se.GetStatus()
}

func TestSharedMapsTheFailuresAnyCommandCanProduce(t *testing.T) {
	t.Parallel()

	assert.Equal(t, http.StatusServiceUnavailable, status(t, apierr.Shared(fmt.Errorf("w: %w", entity.ErrReadModelBehind))))
	assert.Equal(t, http.StatusConflict, status(t, apierr.Shared(fmt.Errorf("w: %w", entity.ErrConcurrencyConflict))))
	assert.Equal(t, http.StatusNotImplemented, status(t, apierr.Shared(fmt.Errorf("w: %w", entity.ErrNotImplemented))))
}

func TestSharedLeavesOtherErrorsToTheOperation(t *testing.T) {
	t.Parallel()

	assert.NoError(t, apierr.Shared(errors.New("anything else"))) //nolint:err113 // a one-off error is the point
	assert.NoError(t, apierr.Shared(entity.ErrTaskNotFound))
}
