package gqlerr_test

import (
	"errors"
	"fmt"
	"testing"

	"github.com/m7medvision/go-clean-template/internal/controller/graphql/gqlerr"
	"github.com/m7medvision/go-clean-template/internal/entity"
	"github.com/m7medvision/go-clean-template/pkg/logger"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/vektah/gqlparser/v2/gqlerror"
)

var errBroken = errors.New("broken")

func code(t *testing.T, err error) string {
	t.Helper()

	var gqlErr *gqlerror.Error
	require.ErrorAs(t, err, &gqlErr)

	return fmt.Sprint(gqlErr.Extensions["code"])
}

func TestMapGivesEachSentinelItsCode(t *testing.T) {
	t.Parallel()

	l := logger.New("error")

	for sentinel, want := range map[error]string{
		entity.ErrTaskNotFound:        "NOT_FOUND",
		entity.ErrTaskForbidden:       "FORBIDDEN",
		entity.ErrAdminRequired:       "FORBIDDEN",
		entity.ErrInvalidTransition:   "BAD_USER_INPUT",
		entity.ErrReadModelBehind:     "UNAVAILABLE",
		entity.ErrConcurrencyConflict: "CONFLICT",
		entity.ErrNotImplemented:      "NOT_IMPLEMENTED",
		errBroken:                     "INTERNAL",
	} {
		assert.Equal(t, want, code(t, gqlerr.Map(l, fmt.Errorf("wrapped: %w", sentinel), "op")), sentinel.Error())
	}
}
