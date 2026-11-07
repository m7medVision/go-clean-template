package task_test

import (
	"context"
	"testing"

	"github.com/m7medvision/go-clean-template/internal/entity"
	"github.com/m7medvision/go-clean-template/internal/repo/persistent/task"
	"github.com/stretchr/testify/require"
)

// An ID that is not a UUID is not found before the read model touches the database, which this one
// does not have.
func TestGetOfAnIDThatIsNotAUUIDIsNotFound(t *testing.T) {
	t.Parallel()

	for name, id := range map[string]string{
		"not hex":         "garbage",
		"empty":           "",
		"one digit short": "6f1c2a4e-3b5d-4c7e-9f80-1a2b3c4d5e6",
	} {
		_, err := task.NewReadModel(nil).Get(context.Background(), id)

		require.ErrorIs(t, err, entity.ErrTaskNotFound, name)
	}
}
