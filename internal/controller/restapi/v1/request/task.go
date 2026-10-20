package request

import "github.com/m7medvision/go-clean-template/internal/entity"

// CreateTask -.
type CreateTask struct {
	_ struct{} `additionalProperties:"true"`

	Title       string `json:"title" minLength:"1" maxLength:"255" example:"My task"`
	Description string `json:"description" required:"false" maxLength:"1000" example:"Task description"`
}

// UpdateTask -.
type UpdateTask struct {
	_ struct{} `additionalProperties:"true"`

	Title       string `json:"title" minLength:"1" maxLength:"255" example:"Updated task"`
	Description string `json:"description" required:"false" maxLength:"1000" example:"Updated description"`
}

// TransitionTask -.
type TransitionTask struct {
	_ struct{} `additionalProperties:"true"`

	Status entity.TaskStatus `json:"status" minLength:"1" enum:"todo,in_progress,done" example:"in_progress"`
}
