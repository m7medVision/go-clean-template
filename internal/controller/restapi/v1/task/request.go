package task

import (
	"encoding/json"
	"reflect"

	"github.com/danielgtaylor/huma/v2"
	"github.com/m7medvision/go-clean-template/internal/entity"
	mongotask "github.com/m7medvision/go-clean-template/internal/entity/mongo/task"
)

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

type UpdateTaskAttributes struct {
	_ struct{} `additionalProperties:"true"`

	Priority Nullable[mongotask.Priority] `json:"priority,omitzero" required:"false" enum:"low,medium,high" doc:"null clears it; left out, it stays"`
	Estimate Nullable[float64]            `json:"estimate,omitzero" required:"false" minimum:"0.5" maximum:"200" doc:"hours; null clears it; left out, it stays"`
	Labels   Nullable[[]string]           `json:"labels,omitzero" required:"false" maxItems:"10" uniqueItems:"true" doc:"null clears them; left out, they stay"`
}

// Nullable tells a key left out of the body (Sent false) from one sent as null (Value nil).
type Nullable[T any] struct {
	Sent  bool
	Value *T
}

func (n *Nullable[T]) UnmarshalJSON(b []byte) error {
	n.Sent = true

	if string(b) == "null" {
		return nil
	}

	var v T
	if err := json.Unmarshal(b, &v); err != nil {
		return err
	}

	n.Value = &v

	return nil
}

func (n Nullable[T]) Schema(r huma.Registry) *huma.Schema {
	var zero T

	s := huma.SchemaFromType(r, reflect.TypeOf(zero))
	s.Nullable = true

	return s
}

func (n Nullable[T]) field() mongotask.Field[T] {
	return mongotask.Field[T]{Set: n.Sent, Value: n.Value}
}

func (b UpdateTaskAttributes) patch() mongotask.AttributesPatch {
	return mongotask.AttributesPatch{Priority: b.Priority.field(), Estimate: b.Estimate.field(), Labels: b.Labels.field()}
}
