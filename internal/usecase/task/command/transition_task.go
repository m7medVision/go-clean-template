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
	cqrs.Command[entity.Written]

	UserID string
	TaskID string
	Status entity.TaskStatus
}

func (c TransitionTask) ActorID() string { return c.UserID }

func (c TransitionTask) Validate() error { return entity.ValidateTaskStatus(c.Status) }

// TransitionTaskHandler's Now is the clock (time.Now when nil).
type TransitionTaskHandler struct {
	Store repo.EventStore
	Now   func() time.Time
}

func (h TransitionTaskHandler) Handle(ctx context.Context, cmd TransitionTask) (entity.Written, error) {
	written, err := aggregate.Change(ctx, h.Store, cmd.TaskID, func(t *entity.TaskAggregate) error {
		return t.Transition(cmd.UserID, cmd.Status, aggregate.Now(h.Now))
	})
	if err != nil {
		return entity.Written{}, fmt.Errorf("TransitionTask: %w", err)
	}

	return written, nil
}
