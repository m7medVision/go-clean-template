package task

import "github.com/m7medvision/go-clean-template/internal/entity"

type CreateTask struct {
	_ struct{} `additionalProperties:"true"`

	Title       string `json:"title" minLength:"1" maxLength:"255" example:"My task"`
	Description string `json:"description" required:"false" maxLength:"1000" example:"Task description"`
}

type UpdateTask struct {
	_ struct{} `additionalProperties:"true"`

	Title       string `json:"title" minLength:"1" maxLength:"255" example:"Updated task"`
	Description string `json:"description" required:"false" maxLength:"1000" example:"Updated description"`
}

type TransitionTask struct {
	_ struct{} `additionalProperties:"true"`

	Status entity.TaskStatus `json:"status" minLength:"1" enum:"todo,in_progress,done" example:"in_progress"`
}

type listTasksInput struct {
	Status entity.TaskStatus `query:"status" required:"false" enum:"todo,in_progress,done" example:"todo"`
	Limit  int               `query:"limit" default:"10" minimum:"1" maximum:"100"`
	Offset int               `query:"offset" default:"0" minimum:"0"`
}

type taskPathInput struct {
	ID string `path:"id" example:"550e8400-e29b-41d4-a716-446655440000"`
}
