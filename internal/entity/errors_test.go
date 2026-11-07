package entity_test

import (
	"errors"
	"fmt"
	"testing"

	"github.com/m7medvision/go-clean-template/internal/entity"
	"github.com/stretchr/testify/assert"
)

func TestKindOfFindsTheFirstDomainErrorInTheChain(t *testing.T) {
	t.Parallel()

	wrapped := fmt.Errorf("handler: %w", entity.ErrTaskNotFound)
	assert.Equal(t, entity.KindNotFound, entity.KindOf(wrapped))

	both := fmt.Errorf("login: %w: %w", entity.ErrPasskeyLoginInvalid, entity.ErrPasskeyCeremonyInvalid)
	assert.Equal(t, entity.KindUnauthenticated, entity.KindOf(both))
}

func TestKindOfAnyOtherErrorIsInternal(t *testing.T) {
	t.Parallel()

	assert.Equal(t, entity.KindInternal, entity.KindOf(errors.New("boom"))) //nolint:err113 // a one-off error is the point
	assert.Equal(t, entity.KindInternal, entity.KindOf(entity.ErrStreamNotFound))
	assert.Equal(t, entity.KindInternal, entity.KindOf(nil))
}
