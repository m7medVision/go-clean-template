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
	cqrs.Command[cqrs.Written]

	UserID      string
	Title       string
	Description string
}

// ActorID -.
func (c CreateTask) ActorID() string { return c.UserID }

// CreateTaskHandler handles CreateTask. Now is the clock (time.Now when nil).
type CreateTaskHandler struct {
	Store repo.EventStore
	Now   func() time.Time
}

// Handle -.
func (h CreateTaskHandler) Handle(ctx context.Context, cmd CreateTask) (cqrs.Written, error) {
	id := uuid.NewString()

	written, err := aggregate.Create(ctx, h.Store, id,
		entity.NewTaskAggregate(id, cmd.UserID, cmd.Title, cmd.Description, aggregate.Now(h.Now)))
	if err != nil {
		return cqrs.Written{}, fmt.Errorf("CreateTask - aggregate.Create: %w", err)
	}

	return written, nil
}
