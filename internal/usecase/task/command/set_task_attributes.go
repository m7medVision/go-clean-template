package command

import (
	"context"
	"fmt"

	"github.com/m7medvision/go-clean-template/internal/entity"
	mongotask "github.com/m7medvision/go-clean-template/internal/entity/mongo/task"
	"github.com/m7medvision/go-clean-template/internal/repo"
	"github.com/m7medvision/go-clean-template/pkg/cqrs"
)

// SetTaskAttributes patches the attributes of UserID's task. It is the one command that writes
// without an aggregate: attributes live in their own store, with no events, so it records none and
// the wait step does not wait.
type SetTaskAttributes struct {
	cqrs.Command[entity.Written]

	UserID string
	TaskID string
	Patch  mongotask.AttributesPatch
}

//nolint:gocritic // the buses pass every message by value
func (c SetTaskAttributes) ActorID() string { return c.UserID }

//nolint:gocritic // the buses pass every message by value
func (c SetTaskAttributes) Validate() error { return c.Patch.Validate() }

type SetTaskAttributesHandler struct {
	Tasks      repo.TaskReadModel
	Attributes repo.TaskAttributesStore
}

//nolint:gocritic // the buses pass every message by value
func (h SetTaskAttributesHandler) Handle(ctx context.Context, cmd SetTaskAttributes) (entity.Written, error) {
	task, err := h.Tasks.Get(ctx, cmd.TaskID)
	if err != nil {
		return entity.Written{}, fmt.Errorf("SetTaskAttributes - h.Tasks.Get: %w", err)
	}

	if err = task.ReadableBy(cmd.UserID); err != nil {
		return entity.Written{}, fmt.Errorf("SetTaskAttributes: %w", err)
	}

	if err = h.Attributes.Patch(ctx, cmd.TaskID, cmd.Patch); err != nil {
		return entity.Written{}, fmt.Errorf("SetTaskAttributes - h.Attributes.Patch: %w", err)
	}

	return entity.Written{ID: cmd.TaskID}, nil
}
