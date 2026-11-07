package entity

import (
	"encoding/json"
	"fmt"
	"time"
)

// _taskSnapshotVersion is the version of taskSnapshot's shape.
const _taskSnapshotVersion = 1

// TaskStreamID is the stream of the task with id.
func TaskStreamID(id string) string {
	return "task-" + id
}

// TaskAggregate is a task on the write side: rebuilt from its events, it checks the task's rules
// and records what changed. Reads use Task, from the table the task projector writes.
type TaskAggregate struct {
	AggregateBase

	task    taskState
	deleted bool
}

// taskState is what the task's rules need. Its JSON is the snapshot's: a field added here is a
// field old snapshots lack, read as its zero value.
type taskState struct {
	ID     string     `json:"id"`
	UserID string     `json:"user_id"`
	Status TaskStatus `json:"status"`
}

var _ Snapshotter = (*TaskAggregate)(nil)

// NewTaskAggregate creates a task for userID, in status todo.
func NewTaskAggregate(id, userID, title, description string, at time.Time) *TaskAggregate {
	t := &TaskAggregate{}
	Record(t, TaskCreated{TaskID: id, UserID: userID, Title: title, Description: description, At: at})

	return t
}

// ImportTaskAggregate records a task written before tasks stored events, whole, as TaskImported.
func ImportTaskAggregate(row *Task) *TaskAggregate {
	t := &TaskAggregate{}
	Record(t, TaskImported{
		TaskID: row.ID, UserID: row.UserID, Title: row.Title, Description: row.Description,
		Status: row.Status, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
	})

	return t
}

// StreamID -.
func (t *TaskAggregate) StreamID() string {
	return TaskStreamID(t.task.ID)
}

// ID is the task's ID.
func (t *TaskAggregate) ID() string {
	return t.task.ID
}

// StreamOf names the stream of the task with id.
func (*TaskAggregate) StreamOf(id string) string {
	return TaskStreamID(id)
}

// NotFound is the error for a task ID with no stream.
func (*TaskAggregate) NotFound() error {
	return ErrTaskNotFound
}

// ownedBy is nil when userID owns the live task: a deleted task is ErrTaskNotFound and another
// user's is ErrTaskForbidden.
func (t *TaskAggregate) ownedBy(userID string) error {
	switch {
	case t.deleted:
		return ErrTaskNotFound
	case t.task.UserID != userID:
		return ErrTaskForbidden
	default:
		return nil
	}
}

// Update replaces the title and description of userID's task.
func (t *TaskAggregate) Update(userID, title, description string, at time.Time) error {
	if err := t.ownedBy(userID); err != nil {
		return err
	}

	Record(t, TaskUpdated{TaskID: t.task.ID, Title: title, Description: description, At: at})

	return nil
}

// Transition moves userID's task to status to, when the task's current status allows it, and is
// ErrInvalidTransition otherwise.
func (t *TaskAggregate) Transition(userID string, to TaskStatus, at time.Time) error {
	if err := t.ownedBy(userID); err != nil {
		return err
	}

	if !t.task.Status.CanMoveTo(to) {
		return fmt.Errorf("%s to %s: %w", t.task.Status, to, ErrInvalidTransition)
	}

	Record(t, TaskTransitioned{TaskID: t.task.ID, From: t.task.Status, To: to, At: at})

	return nil
}

// Delete deletes userID's task. Another user's task is ErrTaskNotFound, not ErrTaskForbidden, so a
// delete never tells a stranger the task exists.
func (t *TaskAggregate) Delete(userID string, at time.Time) error {
	if t.deleted || t.task.UserID != userID {
		return ErrTaskNotFound
	}

	Record(t, TaskDeleted{TaskID: t.task.ID, At: at})

	return nil
}

// Apply -.
func (t *TaskAggregate) Apply(e Event) {
	switch ev := e.(type) {
	case TaskCreated:
		t.task = taskState{ID: ev.TaskID, UserID: ev.UserID, Status: TaskStatusTodo}
	case TaskImported:
		t.task = taskState{ID: ev.TaskID, UserID: ev.UserID, Status: ev.Status}
	case TaskUpdated: // the title and description are the read model's; no rule reads them
	case TaskTransitioned:
		t.task.Status = ev.To
	case TaskDeleted:
		t.deleted = true
	}
}

type taskSnapshot struct {
	Task    taskState `json:"task"`
	Deleted bool      `json:"deleted"`
}

// SnapshotVersion -.
func (*TaskAggregate) SnapshotVersion() int {
	return _taskSnapshotVersion
}

// Snapshot -.
func (t *TaskAggregate) Snapshot() any {
	return taskSnapshot{Task: t.task, Deleted: t.deleted}
}

// RestoreSnapshot -.
func (t *TaskAggregate) RestoreSnapshot(data json.RawMessage) error {
	var s taskSnapshot
	if err := json.Unmarshal(data, &s); err != nil {
		return fmt.Errorf("TaskAggregate - RestoreSnapshot: %w", err)
	}

	t.task, t.deleted = s.Task, s.Deleted

	return nil
}
