// Package task implements the task read model on Postgres (the tasks table), the projector that
// writes it from the task events, and the importer that turns rows written before tasks stored
// events into events.
package task

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/m7medvision/go-clean-template/internal/entity"
	"github.com/m7medvision/go-clean-template/internal/repo"
	"github.com/m7medvision/go-clean-template/internal/repo/persistent/queries"
	"github.com/m7medvision/go-clean-template/pkg/postgres"
)

// Repo reads the tasks table. Its queries join the transaction the context carries.
type Repo struct {
	pg *postgres.Postgres
}

func NewReadModel(pg *postgres.Postgres) repo.TaskReadModel {
	return &Repo{pg: pg}
}

func (r *Repo) q(ctx context.Context) *queries.Queries {
	return queries.New(r.pg.Executor(ctx))
}

func (r *Repo) Get(ctx context.Context, userID, taskID string) (entity.Task, error) {
	task, err := r.q(ctx).GetTask(ctx, taskID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return entity.Task{}, entity.ErrTaskNotFound
		}

		return entity.Task{}, fmt.Errorf("TaskReadModel - Get - r.q(ctx).GetTask: %w", err)
	}

	if task.UserID != userID {
		return entity.Task{}, entity.ErrTaskForbidden
	}

	return entity.Task(task), nil
}

func (r *Repo) List(
	ctx context.Context, userID string, status *entity.TaskStatus, page entity.Page,
) (entity.List[entity.Task], error) {
	total, err := r.q(ctx).CountTasks(ctx, queries.CountTasksParams{UserID: userID, Status: (*string)(status)})
	if err != nil {
		return entity.List[entity.Task]{}, fmt.Errorf("TaskReadModel - List - r.q(ctx).CountTasks: %w", err)
	}

	rows, err := r.q(ctx).ListTasks(ctx, queries.ListTasksParams{
		UserID: userID,
		Status: (*string)(status),
		Limit:  int64(page.Size()),
		Offset: int64(page.Offset),
	})
	if err != nil {
		return entity.List[entity.Task]{}, fmt.Errorf("TaskReadModel - List - r.q(ctx).ListTasks: %w", err)
	}

	return entity.List[entity.Task]{Items: tasks(rows), Total: int(total)}, nil
}

func (r *Repo) ListAll(ctx context.Context, status *entity.TaskStatus, page entity.Page) (entity.List[entity.Task], error) {
	total, err := r.q(ctx).CountAllTasks(ctx, (*string)(status))
	if err != nil {
		return entity.List[entity.Task]{}, fmt.Errorf("TaskReadModel - ListAll - r.q(ctx).CountAllTasks: %w", err)
	}

	rows, err := r.q(ctx).ListAllTasks(ctx, queries.ListAllTasksParams{
		Status: (*string)(status),
		Limit:  int64(page.Size()),
		Offset: int64(page.Offset),
	})
	if err != nil {
		return entity.List[entity.Task]{}, fmt.Errorf("TaskReadModel - ListAll - r.q(ctx).ListAllTasks: %w", err)
	}

	return entity.List[entity.Task]{Items: tasks(rows), Total: int(total)}, nil
}

func tasks(rows []queries.Task) []entity.Task {
	tasks := make([]entity.Task, len(rows))
	for i := range rows {
		tasks[i] = entity.Task(rows[i])
	}

	return tasks
}
