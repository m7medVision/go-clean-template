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

// UpdateTask replaces the title and description of UserID's task. Another user's task is
// entity.ErrTaskForbidden.
type UpdateTask struct {
	cqrs.Command[entity.Written]

	UserID      string
	TaskID      string
	Title       string
	Description string
}

// ActorID -.
func (c UpdateTask) ActorID() string { return c.UserID }

// Validate -.
func (c UpdateTask) Validate() error { return entity.ValidateTaskText(c.Title, c.Description) }

// UpdateTaskHandler handles UpdateTask. Now is the clock (time.Now when nil).
type UpdateTaskHandler struct {
	Store repo.EventStore
	Now   func() time.Time
}

// Handle -.
func (h UpdateTaskHandler) Handle(ctx context.Context, cmd UpdateTask) (entity.Written, error) {
	written, err := aggregate.Change(ctx, h.Store, cmd.TaskID, entity.TaskStreamID(cmd.TaskID), &entity.TaskAggregate{},
		entity.ErrTaskNotFound, func(t *entity.TaskAggregate) error {
			return t.Update(cmd.UserID, cmd.Title, cmd.Description, aggregate.Now(h.Now))
		})
	if err != nil {
		return entity.Written{}, fmt.Errorf("UpdateTask: %w", err)
	}

	return written, nil
}
