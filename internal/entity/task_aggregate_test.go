package entity_test

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/m7medvision/go-clean-template/internal/entity"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	_owner  = "user-1"
	_other  = "user-2"
	_taskID = "task-1"
)

// createdAt and laterAt are when the tests' task is created and when it next changes.
func createdAt() time.Time { return time.Date(2026, 1, 1, 9, 0, 0, 0, time.UTC) }
func laterAt() time.Time   { return createdAt().Add(time.Hour) }

// givenTask is a task rebuilt from events, the way the store loads one.
func givenTask(events ...entity.Event) *entity.TaskAggregate {
	t := &entity.TaskAggregate{}
	entity.Replay(t, len(events), events...)

	return t
}

func created() entity.TaskCreated {
	return entity.TaskCreated{TaskID: _taskID, UserID: _owner, Title: "Write", Description: "the spec", At: createdAt()}
}

func TestNewTaskAggregateRecordsTaskCreated(t *testing.T) {
	t.Parallel()

	task := entity.NewTaskAggregate(_taskID, _owner, "Write", "the spec", createdAt())

	assert.Equal(t, []entity.Event{created()}, task.Pending())
	assert.Equal(t, "task-"+_taskID, task.StreamID())
	assert.Equal(t, entity.Task{
		ID: _taskID, UserID: _owner, Title: "Write", Description: "the spec",
		Status: entity.TaskStatusTodo, CreatedAt: createdAt(), UpdatedAt: createdAt(),
	}, task.State())
}

func TestTaskAggregateUpdateByItsOwner(t *testing.T) {
	t.Parallel()

	task := givenTask(created())

	require.NoError(t, task.Update(_owner, "Edit", "the plan", laterAt()))

	assert.Equal(t, []entity.Event{
		entity.TaskUpdated{TaskID: _taskID, Title: "Edit", Description: "the plan", At: laterAt()},
	}, task.Pending())
	assert.Equal(t, "Edit", task.State().Title)
	assert.Equal(t, laterAt(), task.State().UpdatedAt)
}

func TestTaskAggregateRefusesAnotherUser(t *testing.T) {
	t.Parallel()

	task := givenTask(created())

	require.ErrorIs(t, task.Update(_other, "x", "y", laterAt()), entity.ErrTaskForbidden)
	require.ErrorIs(t, task.Transition(_other, entity.TaskStatusInProgress, laterAt()), entity.ErrTaskForbidden)
	require.ErrorIs(t, task.Delete(_other, laterAt()), entity.ErrTaskNotFound, "a delete never tells a stranger the task exists")
	assert.Empty(t, task.Pending())
}

func TestTaskAggregateTransition(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		from    entity.TaskStatus
		to      entity.TaskStatus
		allowed bool
	}{
		{"todo to in_progress", entity.TaskStatusTodo, entity.TaskStatusInProgress, true},
		{"in_progress to done", entity.TaskStatusInProgress, entity.TaskStatusDone, true},
		{"in_progress to todo", entity.TaskStatusInProgress, entity.TaskStatusTodo, true},
		{"todo to done", entity.TaskStatusTodo, entity.TaskStatusDone, false},
		{"done to todo", entity.TaskStatusDone, entity.TaskStatusTodo, false},
		{"done to in_progress", entity.TaskStatusDone, entity.TaskStatusInProgress, false},
		{"todo to todo", entity.TaskStatusTodo, entity.TaskStatusTodo, false},
		{"to an unknown status", entity.TaskStatusTodo, entity.TaskStatus("unknown"), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			task := givenTask(entity.TaskImported{
				TaskID: _taskID, UserID: _owner, Title: "t", Status: tt.from, CreatedAt: createdAt(), UpdatedAt: createdAt(),
			})

			err := task.Transition(_owner, tt.to, laterAt())

			if !tt.allowed {
				require.ErrorIs(t, err, entity.ErrInvalidTransition)
				assert.Empty(t, task.Pending())
				assert.Equal(t, tt.from, task.State().Status)

				return
			}

			require.NoError(t, err)
			assert.Equal(t, []entity.Event{
				entity.TaskTransitioned{TaskID: _taskID, From: tt.from, To: tt.to, At: laterAt()},
			}, task.Pending())
			assert.Equal(t, tt.to, task.State().Status)
		})
	}
}

func TestTaskAggregateDelete(t *testing.T) {
	t.Parallel()

	task := givenTask(created())

	require.NoError(t, task.Delete(_owner, laterAt()))
	assert.Equal(t, []entity.Event{entity.TaskDeleted{TaskID: _taskID, At: laterAt()}}, task.Pending())
}

func TestADeletedTaskIsNotFound(t *testing.T) {
	t.Parallel()

	task := givenTask(created(), entity.TaskDeleted{TaskID: _taskID, At: laterAt()})

	require.ErrorIs(t, task.Update(_owner, "x", "y", laterAt()), entity.ErrTaskNotFound)
	require.ErrorIs(t, task.Transition(_owner, entity.TaskStatusInProgress, laterAt()), entity.ErrTaskNotFound)
	require.ErrorIs(t, task.Delete(_owner, laterAt()), entity.ErrTaskNotFound)
}

func TestTaskImportedRestoresTheWholeTask(t *testing.T) {
	t.Parallel()

	task := givenTask(entity.TaskImported{
		TaskID: _taskID, UserID: _owner, Title: "Old", Description: "row",
		Status: entity.TaskStatusDone, CreatedAt: createdAt(), UpdatedAt: laterAt(),
	})

	assert.Equal(t, entity.Task{
		ID: _taskID, UserID: _owner, Title: "Old", Description: "row",
		Status: entity.TaskStatusDone, CreatedAt: createdAt(), UpdatedAt: laterAt(),
	}, task.State())
}

func TestTaskAggregateSnapshotRestoresTheSameTask(t *testing.T) {
	t.Parallel()

	task := givenTask(created(), entity.TaskTransitioned{
		TaskID: _taskID, From: entity.TaskStatusTodo, To: entity.TaskStatusInProgress, At: laterAt(),
	})

	data, err := json.Marshal(task.Snapshot())
	require.NoError(t, err)

	restored := &entity.TaskAggregate{}
	require.NoError(t, restored.RestoreSnapshot(data))
	assert.Equal(t, task.State(), restored.State())
}

func TestTheTaskEventsAreRegistered(t *testing.T) {
	t.Parallel()

	names := map[string]bool{}
	for _, e := range entity.EventTypes() {
		names[e.EventType()] = true
	}

	for _, e := range []entity.Event{
		entity.TaskCreated{}, entity.TaskUpdated{}, entity.TaskTransitioned{}, entity.TaskDeleted{}, entity.TaskImported{},
	} {
		assert.True(t, names[e.EventType()], e.EventType())
	}
}

func TestImportTaskAggregateRecordsTheRowAsTaskImported(t *testing.T) {
	t.Parallel()

	row := entity.Task{
		ID: _taskID, UserID: _owner, Title: "Old", Description: "row",
		Status: entity.TaskStatusInProgress, CreatedAt: createdAt(), UpdatedAt: laterAt(),
	}

	task := entity.ImportTaskAggregate(&row)

	assert.Equal(t, []entity.Event{entity.TaskImported{
		TaskID: _taskID, UserID: _owner, Title: "Old", Description: "row",
		Status: entity.TaskStatusInProgress, CreatedAt: createdAt(), UpdatedAt: laterAt(),
	}}, task.Pending())
	assert.Equal(t, row, task.State())
}
