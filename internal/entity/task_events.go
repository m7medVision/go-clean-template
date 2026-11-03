package entity

import "time"

// The task events, all at schema version 1. Each carries the task's ID, so a projector needs no
// other context.
const _taskEventsVersion = 1

// TaskCreated is a task's first event.
type TaskCreated struct {
	TaskID      string    `json:"task_id"`
	UserID      string    `json:"user_id"`
	Title       string    `json:"title"`
	Description string    `json:"description"`
	At          time.Time `json:"at"`
}

// TaskUpdated replaces a task's title and description.
type TaskUpdated struct {
	TaskID      string    `json:"task_id"`
	Title       string    `json:"title"`
	Description string    `json:"description"`
	At          time.Time `json:"at"`
}

// TaskTransitioned moves a task from one status to the next.
type TaskTransitioned struct {
	TaskID string     `json:"task_id"`
	From   TaskStatus `json:"from"`
	To     TaskStatus `json:"to"`
	At     time.Time  `json:"at"`
}

// TaskDeleted ends a task: its stream stays, and loads as not found.
type TaskDeleted struct {
	TaskID string    `json:"task_id"`
	At     time.Time `json:"at"`
}

// TaskImported is a task written before tasks stored events: the backfill's one event per row,
// carrying the row whole.
type TaskImported struct {
	TaskID      string     `json:"task_id"`
	UserID      string     `json:"user_id"`
	Title       string     `json:"title"`
	Description string     `json:"description"`
	Status      TaskStatus `json:"status"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
}

// EventType -.
func (TaskCreated) EventType() string { return "TaskCreated" }

// SchemaVersion -.
func (TaskCreated) SchemaVersion() int { return _taskEventsVersion }

// EventType -.
func (TaskUpdated) EventType() string { return "TaskUpdated" }

// SchemaVersion -.
func (TaskUpdated) SchemaVersion() int { return _taskEventsVersion }

// EventType -.
func (TaskTransitioned) EventType() string { return "TaskTransitioned" }

// SchemaVersion -.
func (TaskTransitioned) SchemaVersion() int { return _taskEventsVersion }

// EventType -.
func (TaskDeleted) EventType() string { return "TaskDeleted" }

// SchemaVersion -.
func (TaskDeleted) SchemaVersion() int { return _taskEventsVersion }

// EventType -.
func (TaskImported) EventType() string { return "TaskImported" }

// SchemaVersion -.
func (TaskImported) SchemaVersion() int { return _taskEventsVersion }

func taskEvents() []Event {
	return []Event{TaskCreated{}, TaskUpdated{}, TaskTransitioned{}, TaskDeleted{}, TaskImported{}}
}
