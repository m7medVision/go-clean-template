package entity

import (
	"time"
	"unicode/utf8"
)

var (
	ErrTaskNotFound           = newError(KindNotFound, "task not found")
	ErrTaskForbidden          = newError(KindForbidden, "forbidden")
	ErrInvalidTaskTitle       = newError(KindInvalid, "title must be 1–255 characters")
	ErrInvalidTaskDescription = newError(KindInvalid, "description must be at most 1000 characters")
	ErrInvalidTaskStatus      = newError(KindInvalid, "invalid task status")
	ErrInvalidTransition      = newError(KindInvalid, "invalid status transition")
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

func (s TaskStatus) Valid() bool {
	switch s {
	case TaskStatusTodo, TaskStatusInProgress, TaskStatusDone:
		return true
	default:
		return false
	}
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
