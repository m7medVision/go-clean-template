package exchange_test

import (
	"context"
	"testing"

	"github.com/m7medvision/go-clean-template/internal/entity"
	"github.com/m7medvision/go-clean-template/internal/repo/persistent/exchange"
	"github.com/stretchr/testify/require"
)

// An ID that is not a UUID is not found before the read model touches the database, which this one
// does not have.
func TestConversionOfAnIDThatIsNotAUUIDIsNotFound(t *testing.T) {
	t.Parallel()

	_, err := exchange.NewReadModel(nil).Conversion(context.Background(), "garbage")

	require.ErrorIs(t, err, entity.ErrConversionNotFound)
}
