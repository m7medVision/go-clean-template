package task_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/m7medvision/go-clean-template/internal/entity"
	"github.com/m7medvision/go-clean-template/internal/repo/persistent/queries"
	"github.com/m7medvision/go-clean-template/internal/repo/persistent/task"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var errWrite = errors.New("write failed")

// fakeQuerier records the task writes; any other query panics on the nil embedded interface.
type fakeQuerier struct {
	queries.Querier

	inserted []queries.InsertTaskRowParams
	texts    []queries.SetTaskTextParams
	statuses []queries.SetTaskStatusParams
	deleted  []string
	err      error
}

//nolint:gocritic // the signature is sqlc's Querier, which takes its params by value
func (f *fakeQuerier) InsertTaskRow(_ context.Context, arg queries.InsertTaskRowParams) error {
	f.inserted = append(f.inserted, arg)

	return f.err
}

func (f *fakeQuerier) SetTaskText(_ context.Context, arg queries.SetTaskTextParams) error {
	f.texts = append(f.texts, arg)

	return f.err
}

func (f *fakeQuerier) SetTaskStatus(_ context.Context, arg queries.SetTaskStatusParams) error {
	f.statuses = append(f.statuses, arg)

	return f.err
}

func (f *fakeQuerier) DeleteTaskRow(_ context.Context, id string) error {
	f.deleted = append(f.deleted, id)

	return f.err
}

func at() time.Time { return time.Date(2026, 1, 1, 9, 0, 0, 0, time.UTC) }

func project(t *testing.T, q *fakeQuerier, e entity.Event) error {
	t.Helper()

	return task.Project(context.Background(), q, &entity.RecordedEvent{Position: 1, StreamID: "task-t1", Event: e})
}

func TestTaskCreatedInsertsATodoRow(t *testing.T) {
	t.Parallel()

	q := &fakeQuerier{}

	require.NoError(t, project(t, q, entity.TaskCreated{TaskID: "t1", UserID: "u1", Title: "a", Description: "b", At: at()}))

	assert.Equal(t, []queries.InsertTaskRowParams{{
		ID: "t1", UserID: "u1", Title: "a", Description: "b", Status: entity.TaskStatusTodo, CreatedAt: at(), UpdatedAt: at(),
	}}, q.inserted)
}

func TestTaskImportedInsertsTheRowItCarries(t *testing.T) {
	t.Parallel()

	q := &fakeQuerier{}
	later := at().Add(time.Hour)

	require.NoError(t, project(t, q, entity.TaskImported{
		TaskID: "t1", UserID: "u1", Title: "a", Description: "b", Status: entity.TaskStatusDone, CreatedAt: at(), UpdatedAt: later,
	}))

	assert.Equal(t, []queries.InsertTaskRowParams{{
		ID: "t1", UserID: "u1", Title: "a", Description: "b", Status: entity.TaskStatusDone, CreatedAt: at(), UpdatedAt: later,
	}}, q.inserted)
}

func TestTaskUpdatedSetsTheText(t *testing.T) {
	t.Parallel()

	q := &fakeQuerier{}

	require.NoError(t, project(t, q, entity.TaskUpdated{TaskID: "t1", Title: "a", Description: "b", At: at()}))

	assert.Equal(t, []queries.SetTaskTextParams{{ID: "t1", Title: "a", Description: "b", UpdatedAt: at()}}, q.texts)
}

func TestTaskTransitionedSetsTheStatus(t *testing.T) {
	t.Parallel()

	q := &fakeQuerier{}

	require.NoError(t, project(t, q, entity.TaskTransitioned{
		TaskID: "t1", From: entity.TaskStatusTodo, To: entity.TaskStatusInProgress, At: at(),
	}))

	assert.Equal(t, []queries.SetTaskStatusParams{{ID: "t1", Status: entity.TaskStatusInProgress, UpdatedAt: at()}}, q.statuses)
}

func TestTaskDeletedDeletesTheRow(t *testing.T) {
	t.Parallel()

	q := &fakeQuerier{}

	require.NoError(t, project(t, q, entity.TaskDeleted{TaskID: "t1", At: at()}))

	assert.Equal(t, []string{"t1"}, q.deleted)
}

func TestTheProjectorReturnsAWriteFailure(t *testing.T) {
	t.Parallel()

	err := project(t, &fakeQuerier{err: errWrite}, entity.TaskDeleted{TaskID: "t1"})

	require.ErrorIs(t, err, errWrite)
}

func TestTheTaskProjectorHasACaseForEveryEventItHandles(t *testing.T) {
	t.Parallel()

	for _, e := range task.NewProjector(nil).Handles() {
		require.NotErrorIs(t, project(t, &fakeQuerier{}, e), entity.ErrUnhandledEvent, e.EventType())
	}

	require.ErrorIs(t, project(t, &fakeQuerier{}, otherDomainEvent()), entity.ErrUnhandledEvent)
}

func otherDomainEvent() entity.Event { return entity.UserRegistered{} }
