package request

import "github.com/evrone/go-clean-template/internal/entity"

// CreateTask -.
type CreateTask struct {
	Title       string `json:"title" minLength:"1" maxLength:"255" example:"My task"`
	Description string `json:"description" required:"false" maxLength:"1000" example:"Task description"`
}

// UpdateTask -.
type UpdateTask struct {
	Title       string `json:"title" minLength:"1" maxLength:"255" example:"Updated task"`
	Description string `json:"description" required:"false" maxLength:"1000" example:"Updated description"`
}

// TransitionTask -.
type TransitionTask struct {
	Status entity.TaskStatus `json:"status" minLength:"1" enum:"todo,in_progress,done" example:"in_progress"`
}
