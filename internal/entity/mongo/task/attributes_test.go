package task_test

import (
	"strings"
	"testing"

	"github.com/m7medvision/go-clean-template/internal/entity"
	mongotask "github.com/m7medvision/go-clean-template/internal/entity/mongo/task"
	"github.com/stretchr/testify/assert"
)

func TestAttributesPatchValidate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		patch mongotask.AttributesPatch
		want  error
	}{
		{"nothing", mongotask.AttributesPatch{}, nil},
		{"every field cleared", mongotask.AttributesPatch{
			Priority: mongotask.Cleared[mongotask.Priority](),
			Estimate: mongotask.Cleared[float64](),
			Labels:   mongotask.Cleared[[]string](),
		}, nil},
		{"a valid value each", mongotask.AttributesPatch{
			Priority: mongotask.SetTo(mongotask.PriorityHigh),
			Estimate: mongotask.SetTo(0.5),
			Labels:   mongotask.SetTo([]string{"a", strings.Repeat("b", mongotask.MaxLabelLength)}),
		}, nil},
		{"an unknown priority", mongotask.AttributesPatch{Priority: mongotask.SetTo(mongotask.Priority("urgent"))}, entity.ErrInvalidTaskPriority},
		{"an estimate under the minimum", mongotask.AttributesPatch{Estimate: mongotask.SetTo(0.4)}, entity.ErrInvalidTaskEstimate},
		{"an estimate over the maximum", mongotask.AttributesPatch{Estimate: mongotask.SetTo(200.5)}, entity.ErrInvalidTaskEstimate},
		{"too many labels", mongotask.AttributesPatch{Labels: mongotask.SetTo(make11())}, entity.ErrInvalidTaskLabels},
		{"an empty label", mongotask.AttributesPatch{Labels: mongotask.SetTo([]string{""})}, entity.ErrInvalidTaskLabels},
		{"a label too long", mongotask.AttributesPatch{Labels: mongotask.SetTo([]string{strings.Repeat("x", mongotask.MaxLabelLength+1)})}, entity.ErrInvalidTaskLabels},
		{"a duplicate label", mongotask.AttributesPatch{Labels: mongotask.SetTo([]string{"a", "a"})}, entity.ErrInvalidTaskLabels},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assert.ErrorIs(t, tt.patch.Validate(), tt.want)
		})
	}
}

func make11() []string {
	labels := make([]string, mongotask.MaxLabels+1)
	for i := range labels {
		labels[i] = string(rune('a' + i))
	}

	return labels
}

func TestAttributesPatchChanges(t *testing.T) {
	t.Parallel()

	set, unset := mongotask.AttributesPatch{
		Priority: mongotask.SetTo(mongotask.PriorityLow),
		Estimate: mongotask.Cleared[float64](),
	}.Changes()

	assert.Equal(t, map[string]any{"priority": mongotask.PriorityLow}, set)
	assert.Equal(t, []string{"estimate"}, unset)
	assert.True(t, mongotask.AttributesPatch{}.Empty())
}
