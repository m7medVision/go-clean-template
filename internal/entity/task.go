package entity

import (
	"time"
	"unicode/utf8"
)

const (
	// MaxTaskTitleLen is the longest title a task may have, in characters; the shortest is 1.
	MaxTaskTitleLen = 255
	// MaxTaskDescriptionLen is the longest description a task may have, in characters.
	MaxTaskDescriptionLen = 1000
)

// TaskStatus -.
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

// Valid reports whether s is a known task status.
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

// CanMoveTo reports whether a task in status s may move to status to.
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
