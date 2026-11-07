package task_test

import (
	"context"
	"testing"

	"github.com/m7medvision/go-clean-template/internal/entity"
	"github.com/m7medvision/go-clean-template/internal/repo/persistent/task"
	"github.com/stretchr/testify/require"
)

// An ID that is not a UUID as the app writes it is not found before the read model touches the database, which this one
// does not have.
func TestGetOfAnIDThatIsNotAUUIDIsNotFound(t *testing.T) {
	t.Parallel()

	for name, id := range map[string]string{
		"not hex":         "garbage",
		"empty":           "",
		"one digit short": "6f1c2a4e-3b5d-4c7e-9f80-1a2b3c4d5e6",
		"upper case":      "6F1C2A4E-3B5D-4C7E-9F80-1A2B3C4D5E6F",
		"urn":             "urn:uuid:6f1c2a4e-3b5d-4c7e-9f80-1a2b3c4d5e6f",
	} {
		_, err := task.NewReadModel(nil).Get(context.Background(), id)

		require.ErrorIs(t, err, entity.ErrTaskNotFound, name)
	}
}
