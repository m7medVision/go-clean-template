package task

import (
	"slices"
	"unicode/utf8"

	"github.com/m7medvision/go-clean-template/internal/entity"
)

type Priority string

const (
	PriorityLow    Priority = "low"
	PriorityMedium Priority = "medium"
	PriorityHigh   Priority = "high"
)

const (
	MinEstimate    = 0.5
	MaxEstimate    = 200
	MaxLabels      = 10
	MaxLabelLength = 30
)

func Priorities() []Priority {
	return []Priority{PriorityLow, PriorityMedium, PriorityHigh}
}

func (p Priority) Valid() bool {
	return slices.Contains(Priorities(), p)
}

type Attributes struct {
	Priority *Priority `json:"priority,omitempty" bson:"priority,omitempty"`
	Estimate *float64  `json:"estimate,omitempty" bson:"estimate,omitempty"`
	Labels   []string  `json:"labels,omitempty"   bson:"labels,omitempty"`
}

type TaskWithAttributes struct { //nolint:revive // named beside entity.Task, which it extends
	entity.Task

	Attributes Attributes `json:"attributes"`
}

type Field[T any] struct {
	Set   bool
	Value *T
}

func SetTo[T any](v T) Field[T] { return Field[T]{Set: true, Value: &v} }

func Cleared[T any]() Field[T] { return Field[T]{Set: true} }

type AttributesPatch struct {
	Priority Field[Priority]
	Estimate Field[float64]
	Labels   Field[[]string]
}

func (p AttributesPatch) Empty() bool {
	return !p.Priority.Set && !p.Estimate.Set && !p.Labels.Set
}

func (p AttributesPatch) Validate() error {
	if p.Priority.Value != nil && !p.Priority.Value.Valid() {
		return entity.ErrInvalidTaskPriority
	}

	if p.Estimate.Value != nil && (*p.Estimate.Value < MinEstimate || *p.Estimate.Value > MaxEstimate) {
		return entity.ErrInvalidTaskEstimate
	}

	if p.Labels.Value != nil {
		return validateLabels(*p.Labels.Value)
	}

	return nil
}

func validateLabels(labels []string) error {
	if len(labels) > MaxLabels {
		return entity.ErrInvalidTaskLabels
	}

	seen := make(map[string]struct{}, len(labels))

	for _, l := range labels {
		if n := utf8.RuneCountInString(l); n == 0 || n > MaxLabelLength {
			return entity.ErrInvalidTaskLabels
		}

		if _, dup := seen[l]; dup {
			return entity.ErrInvalidTaskLabels
		}

		seen[l] = struct{}{}
	}

	return nil
}

func (p AttributesPatch) Changes() (set map[string]any, unset []string) {
	set = make(map[string]any)

	change(set, &unset, "priority", p.Priority)
	change(set, &unset, "estimate", p.Estimate)
	change(set, &unset, "labels", p.Labels)

	return set, unset
}

func change[T any](set map[string]any, unset *[]string, key string, f Field[T]) {
	switch {
	case !f.Set:
	case f.Value == nil:
		*unset = append(*unset, key)
	default:
		set[key] = *f.Value
	}
}
