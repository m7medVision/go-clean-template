package command

import (
	"context"
	"fmt"
	"log/slog"
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

func (c DeleteTask) ActorID() string { return c.UserID }

// DeleteTaskHandler's Now is the clock (time.Now when nil). Attributes are deleted after the task;
// a failure there is logged, not returned: the task is gone, and an orphan document is unreachable.
type DeleteTaskHandler struct {
	Store      repo.EventStore
	Attributes repo.TaskAttributesStore
	Logger     *slog.Logger
	Now        func() time.Time
}

func (h DeleteTaskHandler) Handle(ctx context.Context, cmd DeleteTask) (entity.Written, error) {
	written, err := aggregate.Change(ctx, h.Store, cmd.TaskID, func(t *entity.TaskAggregate) error {
		return t.Delete(cmd.UserID, aggregate.Now(h.Now))
	})
	if err != nil {
		return entity.Written{}, fmt.Errorf("DeleteTask: %w", err)
	}

	if err = h.Attributes.Delete(ctx, cmd.TaskID); err != nil {
		h.Logger.ErrorContext(ctx, "DeleteTask - h.Attributes.Delete", "task", cmd.TaskID, "error", err)
	}

	return written, nil
}
