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

// DeleteTask deletes UserID's task. Another user's task is entity.ErrTaskNotFound.
type DeleteTask struct {
	cqrs.Command[entity.Written]

	UserID string
	TaskID string
}

// ActorID -.
func (c DeleteTask) ActorID() string { return c.UserID }

// DeleteTaskHandler handles DeleteTask. Now is the clock (time.Now when nil).
type DeleteTaskHandler struct {
	Store repo.EventStore
	Now   func() time.Time
}

// Handle -.
func (h DeleteTaskHandler) Handle(ctx context.Context, cmd DeleteTask) (entity.Written, error) {
	written, err := aggregate.Change(ctx, h.Store, cmd.TaskID, entity.TaskStreamID(cmd.TaskID), &entity.TaskAggregate{},
		entity.ErrTaskNotFound, func(t *entity.TaskAggregate) error {
			return t.Delete(cmd.UserID, aggregate.Now(h.Now))
		})
	if err != nil {
		return entity.Written{}, fmt.Errorf("DeleteTask: %w", err)
	}

	return written, nil
}
