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

// Repo reads the tasks table.
type Repo struct {
	q *queries.Queries
}

// NewReadModel returns the task read model.
func NewReadModel(pg *postgres.Postgres) repo.TaskReadModel {
	return &Repo{queries.New(pg.Pool)}
}

// Get -.
func (r *Repo) Get(ctx context.Context, userID, taskID string) (entity.Task, error) {
	task, err := r.q.GetTask(ctx, taskID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return entity.Task{}, entity.ErrTaskNotFound
		}

		return entity.Task{}, fmt.Errorf("TaskReadModel - Get - r.q.GetTask: %w", err)
	}

	if task.UserID != userID {
		return entity.Task{}, entity.ErrTaskForbidden
	}

	return entity.Task(task), nil
}

// List -.
func (r *Repo) List(
	ctx context.Context, userID string, status *entity.TaskStatus, page entity.Page,
) (entity.List[entity.Task], error) {
	total, err := r.q.CountTasks(ctx, queries.CountTasksParams{UserID: userID, Status: (*string)(status)})
	if err != nil {
		return entity.List[entity.Task]{}, fmt.Errorf("TaskReadModel - List - r.q.CountTasks: %w", err)
	}

	rows, err := r.q.ListTasks(ctx, queries.ListTasksParams{
		UserID: userID,
		Status: (*string)(status),
		Limit:  int64(page.Size()),
		Offset: int64(page.Offset),
	})
	if err != nil {
		return entity.List[entity.Task]{}, fmt.Errorf("TaskReadModel - List - r.q.ListTasks: %w", err)
	}

	return entity.List[entity.Task]{Items: tasks(rows), Total: int(total)}, nil
}

// ListAll -.
func (r *Repo) ListAll(ctx context.Context, status *entity.TaskStatus, page entity.Page) (entity.List[entity.Task], error) {
	total, err := r.q.CountAllTasks(ctx, (*string)(status))
	if err != nil {
		return entity.List[entity.Task]{}, fmt.Errorf("TaskReadModel - ListAll - r.q.CountAllTasks: %w", err)
	}

	rows, err := r.q.ListAllTasks(ctx, queries.ListAllTasksParams{
		Status: (*string)(status),
		Limit:  int64(page.Size()),
		Offset: int64(page.Offset),
	})
	if err != nil {
		return entity.List[entity.Task]{}, fmt.Errorf("TaskReadModel - ListAll - r.q.ListAllTasks: %w", err)
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
