package command

import (
	"context"
	"fmt"
	"time"

	"github.com/m7medvision/go-clean-template/internal/entity"
	"github.com/m7medvision/go-clean-template/internal/repo"
	"github.com/m7medvision/go-clean-template/internal/usecase/aggregate"
	"github.com/m7medvision/go-clean-template/pkg/cqrs"
)

// TransitionTask moves UserID's task to Status. A move the task's status does not allow is
// entity.ErrInvalidTransition.
type TransitionTask struct {
	UserID string
	TaskID string
	Status entity.TaskStatus
}

// CommandName -.
func (TransitionTask) CommandName() string { return "TransitionTask" }

// ActorID -.
func (c TransitionTask) ActorID() string { return c.UserID }

// TransitionTaskHandler handles TransitionTask. Now is the clock (time.Now when nil).
type TransitionTaskHandler struct {
	Store repo.EventStore
	Now   func() time.Time
}

// Handle -.
func (h TransitionTaskHandler) Handle(ctx context.Context, cmd TransitionTask) (cqrs.Written, error) {
	written, err := aggregate.Change(ctx, h.Store, cmd.TaskID, entity.TaskStreamID(cmd.TaskID), &entity.TaskAggregate{},
		entity.ErrTaskNotFound, func(t *entity.TaskAggregate) error {
			return t.Transition(cmd.UserID, cmd.Status, aggregate.Now(h.Now))
		})
	if err != nil {
		return cqrs.Written{}, fmt.Errorf("TransitionTask: %w", err)
	}

	return written, nil
}
