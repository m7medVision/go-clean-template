package entity

import (
	"fmt"
	"slices"
	"time"
	"unicode/utf8"
)

var (
	ErrTaskNotFound           = newError(KindNotFound, "task not found")
	ErrTaskForbidden          = newError(KindForbidden, "forbidden")
	ErrInvalidTaskTitle       = newError(KindInvalid, fmt.Sprintf("title must be 1–%d characters", MaxTaskTitleLen))
	ErrInvalidTaskDescription = newError(KindInvalid, fmt.Sprintf("description must be at most %d characters", MaxTaskDescriptionLen))
	ErrInvalidTaskStatus      = newError(KindInvalid, "invalid task status")
	ErrInvalidTransition      = newError(KindInvalid, "invalid status transition")
	ErrInvalidTaskPriority    = newError(KindInvalid, "priority must be low, medium or high")
	ErrInvalidTaskEstimate    = newError(KindInvalid, "estimate must be 0.5–200 hours")
	ErrInvalidTaskLabels      = newError(KindInvalid, "labels must be at most 10, each 1–30 characters, with no duplicates")
	ErrUnknownTaskAttribute   = newError(KindInvalid, "unknown attribute: the attributes are priority, estimate and labels")
)

const (
	// MaxTaskTitleLen is the longest title a task may have, in characters; the shortest is 1.
	MaxTaskTitleLen = 255
	// MaxTaskDescriptionLen is the longest description a task may have, in characters.
	MaxTaskDescriptionLen = 1000
)

type TaskStatus string

const (
	TaskStatusTodo       TaskStatus = "todo"
	TaskStatusInProgress TaskStatus = "in_progress"
	TaskStatusDone       TaskStatus = "done"
)

// Task is a task as it is read: the shape the tasks table, and the API, hold. Its rules live on
// TaskAggregate.
type Task struct {
	ID          string     `json:"id"          example:"550e8400-e29b-41d4-a716-446655440000"`
	UserID      string     `json:"user_id"     example:"550e8400-e29b-41d4-a716-446655440000"`
	Title       string     `json:"title"       example:"My task"`
	Description string     `json:"description" example:"Task description"`
	Status      TaskStatus `json:"status"      example:"todo" enum:"todo,in_progress,done"`
	CreatedAt   time.Time  `json:"created_at"  example:"2026-01-01T00:00:00Z"`
	UpdatedAt   time.Time  `json:"updated_at"  example:"2026-01-01T00:00:00Z"`
}

// ReadableBy is nil when userID may read the task: when it is theirs. Another user's task is
// ErrTaskForbidden.
func (t *Task) ReadableBy(userID string) error {
	return taskOwnedBy(t.UserID, userID)
}

// taskOwnedBy is who may read or change a task: its owner. Anyone else is ErrTaskForbidden.
func taskOwnedBy(owner, userID string) error {
	if owner != userID {
		return ErrTaskForbidden
	}

	return nil
}

// TaskStatuses is every status a task can have, in the order a task moves through them. Each
// transport's status enum lists the same values; internal/app and the GraphQL schema test pin them.
func TaskStatuses() []TaskStatus {
	return []TaskStatus{TaskStatusTodo, TaskStatusInProgress, TaskStatusDone}
}

func (s TaskStatus) Valid() bool {
	return slices.Contains(TaskStatuses(), s)
}

// TaskStatusFilter is the status a list of tasks asks for, from a transport's optional field: nil,
// meaning any status, when s is empty.
func TaskStatusFilter(s TaskStatus) *TaskStatus {
	if s == "" {
		return nil
	}

	return &s
}

// ValidateTaskText refuses a title of no characters or more than MaxTaskTitleLen, and a description
// longer than MaxTaskDescriptionLen.
func ValidateTaskText(title, description string) error {
	if n := utf8.RuneCountInString(title); n < 1 || n > MaxTaskTitleLen {
		return ErrInvalidTaskTitle
	}

	if utf8.RuneCountInString(description) > MaxTaskDescriptionLen {
		return ErrInvalidTaskDescription
	}

	return nil
}

// ValidateTaskStatus refuses a status that is not one of the three.
func ValidateTaskStatus(s TaskStatus) error {
	if !s.Valid() {
		return ErrInvalidTaskStatus
	}

	return nil
}

func (s TaskStatus) CanMoveTo(to TaskStatus) bool {
	switch s {
	case TaskStatusTodo:
		return to == TaskStatusInProgress
	case TaskStatusInProgress:
		return to == TaskStatusDone || to == TaskStatusTodo
	case TaskStatusDone:
		return false
	default:
		return false
	}
}
