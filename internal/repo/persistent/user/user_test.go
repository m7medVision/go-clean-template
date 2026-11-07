package user_test

import (
	"context"
	"testing"

	"github.com/m7medvision/go-clean-template/internal/entity"
	"github.com/m7medvision/go-clean-template/internal/repo/persistent/user"
	"github.com/stretchr/testify/require"
)

// An ID that is not a UUID is not found before the read model touches the database, which this one
// does not have.
func TestByIDOfAnIDThatIsNotAUUIDIsNotFound(t *testing.T) {
	t.Parallel()

	_, err := user.NewReadModel(nil).ByID(context.Background(), "garbage")

	require.ErrorIs(t, err, entity.ErrUserNotFound)
}
