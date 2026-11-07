package task_test

import (
	"context"
	"testing"

	"github.com/m7medvision/go-clean-template/internal/entity"
	"github.com/m7medvision/go-clean-template/internal/repo/persistent/queries"
	"github.com/m7medvision/go-clean-template/internal/repo/persistent/task"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// rows is a tasks table holding legacy rows; any other query panics on the nil embedded interface.
type rows struct {
	queries.Querier

	tasks []queries.Task
	err   error
}

func (r rows) ListEveryTask(context.Context) ([]queries.Task, error) { return r.tasks, r.err }

func row(id string) queries.Task {
	return queries.Task{ID: id, UserID: "u1", Title: id, Status: entity.TaskStatusDone, CreatedAt: at(), UpdatedAt: at()}
}

func TestEachTaskRowBecomesAStreamHoldingTaskImported(t *testing.T) {
	t.Parallel()

	imports, err := task.Imports(context.Background(), rows{tasks: []queries.Task{row("t1"), row("t2")}})
	require.NoError(t, err)
	require.Len(t, imports, 2)

	for i, id := range []string{"t1", "t2"} {
		assert.Equal(t, entity.TaskStreamID(id), imports[i].Aggregate.StreamID())
		assert.Equal(t, []entity.Event{entity.TaskImported{
			TaskID: id, UserID: "u1", Title: id, Status: entity.TaskStatusDone, CreatedAt: at(), UpdatedAt: at(),
		}}, imports[i].Aggregate.Pending())
		assert.Empty(t, imports[i].Claims)
	}
}

func TestTheTaskImporterReturnsAReadFailure(t *testing.T) {
	t.Parallel()

	_, err := task.Imports(context.Background(), rows{err: errWrite})

	require.ErrorIs(t, err, errWrite)
}
