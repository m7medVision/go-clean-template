package task_test

import (
	"context"
	"testing"

	"github.com/m7medvision/go-clean-template/internal/entity"
	"github.com/m7medvision/go-clean-template/internal/repo/memstore"
	"github.com/m7medvision/go-clean-template/internal/repo/persistent/task"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func row(id string) entity.Task {
	return entity.Task{ID: id, UserID: "u1", Title: id, Status: entity.TaskStatusDone, CreatedAt: at(), UpdatedAt: at()}
}

func TestImportTasksGivesEachRowItsStream(t *testing.T) {
	t.Parallel()

	store := memstore.New(entity.EventTypes())

	require.NoError(t, task.ImportTasks(context.Background(), store, []entity.Task{row("t1"), row("t2")}))

	for _, id := range []string{"t1", "t2"} {
		r := row(id)
		assert.Equal(t, []entity.Event{entity.TaskImported{
			TaskID: id, UserID: "u1", Title: id, Status: entity.TaskStatusDone, CreatedAt: at(), UpdatedAt: at(),
		}}, store.Events(entity.TaskStreamID(r.ID)))
	}
}

func TestImportTasksSkipsATaskThatHasAStreamAlready(t *testing.T) {
	t.Parallel()

	store := memstore.New(entity.EventTypes())
	_, err := store.Save(context.Background(), entity.NewTaskAggregate("t1", "u1", "new", "", at()))
	require.NoError(t, err)

	require.NoError(t, task.ImportTasks(context.Background(), store, []entity.Task{row("t1")}))

	assert.Len(t, store.Events(entity.TaskStreamID("t1")), 1, "the stream keeps its own first event")
}

func TestTheImporterIsTheTaskDomains(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "task", task.NewImporter(nil, nil).Domain())
}
