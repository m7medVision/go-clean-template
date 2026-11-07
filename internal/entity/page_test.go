package entity_test

import (
	"testing"

	"github.com/m7medvision/go-clean-template/internal/entity"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPageAcceptsAnUnsetOrBoundedLimit(t *testing.T) {
	t.Parallel()

	for _, p := range []entity.Page{{}, {Limit: 1}, {Limit: entity.MaxPageLimit, Offset: 500}} {
		require.NoError(t, p.Validate(), p)
	}

	for _, p := range []entity.Page{{Limit: -1}, {Limit: entity.MaxPageLimit + 1}, {Offset: -1}} {
		require.ErrorIs(t, p.Validate(), entity.ErrInvalidPage, p)
	}
}

func TestAnUnsetLimitIsTheDefaultPage(t *testing.T) {
	t.Parallel()

	assert.Equal(t, entity.DefaultPageLimit, entity.Page{}.Size())
	assert.Equal(t, 3, entity.Page{Limit: 3}.Size())
}
