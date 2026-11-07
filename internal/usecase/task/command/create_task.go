// Package command holds the task domain's commands, one file each, with their handlers.
package command

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/m7medvision/go-clean-template/internal/entity"
	"github.com/m7medvision/go-clean-template/internal/repo"
	"github.com/m7medvision/go-clean-template/internal/usecase/aggregate"
	"github.com/m7medvision/go-clean-template/pkg/cqrs"
)

// CreateTask creates a task owned by UserID, in status todo.
type CreateTask struct {
	cqrs.Command[entity.Written]

	UserID      string
	Title       string
	Description string
}

func (c CreateTask) ActorID() string { return c.UserID }

func (c CreateTask) Validate() error { return entity.ValidateTaskText(c.Title, c.Description) }

// CreateTaskHandler's Now is the clock (time.Now when nil).
type CreateTaskHandler struct {
	Store repo.EventStore
	Now   func() time.Time
}

func (h CreateTaskHandler) Handle(ctx context.Context, cmd CreateTask) (entity.Written, error) {
	id := uuid.NewString()

	written, err := aggregate.Save(ctx, h.Store, entity.NewTaskAggregate(id, cmd.UserID, cmd.Title, cmd.Description, aggregate.Now(h.Now)))
	if err != nil {
		return entity.Written{}, fmt.Errorf("CreateTask - aggregate.Save: %w", err)
	}

	return written, nil
}
