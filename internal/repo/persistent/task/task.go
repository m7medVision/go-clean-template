// Package task implements the Postgres-backed Task repository.
package task

import (
	"context"
	"errors"
	"fmt"

	"github.com/evrone/go-clean-template/internal/entity"
	"github.com/evrone/go-clean-template/internal/repo"
	"github.com/evrone/go-clean-template/internal/repo/persistent/queries"
	"github.com/evrone/go-clean-template/pkg/postgres"
	"github.com/jackc/pgx/v5"
)

// Repo -.
type Repo struct {
	q *queries.Queries
}

// New returns a Task repository instrumented with OpenTelemetry tracing spans.
func New(pg *postgres.Postgres) repo.TaskRepo {
	return newTraced(&Repo{queries.New(pg.Pool)})
}

// Store -.
func (r *Repo) Store(ctx context.Context, task *entity.Task) error {
	if err := r.q.CreateTask(ctx, queries.CreateTaskParams(*task)); err != nil {
		return fmt.Errorf("TaskRepo - Store - r.q.CreateTask: %w", err)
	}

	return nil
}

// GetByID -.
func (r *Repo) GetByID(ctx context.Context, userID, taskID string) (entity.Task, error) {
	task, err := r.q.GetTask(ctx, taskID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return entity.Task{}, entity.ErrTaskNotFound
		}

		return entity.Task{}, fmt.Errorf("TaskRepo - GetByID - r.q.GetTask: %w", err)
	}

	if task.UserID != userID {
		return entity.Task{}, entity.ErrTaskForbidden
	}

	return entity.Task(task), nil
}

// List -.
func (r *Repo) List(ctx context.Context, userID string, filter repo.TaskFilter) ([]entity.Task, int, error) {
	status := (*string)(filter.Status)

	total, err := r.q.CountTasks(ctx, queries.CountTasksParams{UserID: userID, Status: status})
	if err != nil {
		return nil, 0, fmt.Errorf("TaskRepo - List - r.q.CountTasks: %w", err)
	}

	rows, err := r.q.ListTasks(ctx, queries.ListTasksParams{
		UserID: userID,
		Status: status,
		Limit:  filter.Limit,
		Offset: filter.Offset,
	})
	if err != nil {
		return nil, 0, fmt.Errorf("TaskRepo - List - r.q.ListTasks: %w", err)
	}

	tasks := make([]entity.Task, len(rows))
	for i := range rows {
		tasks[i] = entity.Task(rows[i])
	}

	return tasks, int(total), nil
}

// Update -.
func (r *Repo) Update(ctx context.Context, task *entity.Task) error {
	affected, err := r.q.UpdateTask(ctx, queries.UpdateTaskParams{
		ID:          task.ID,
		UserID:      task.UserID,
		Title:       task.Title,
		Description: task.Description,
		Status:      task.Status,
		UpdatedAt:   task.UpdatedAt,
	})
	if err != nil {
		return fmt.Errorf("TaskRepo - Update - r.q.UpdateTask: %w", err)
	}

	if affected == 0 {
		return entity.ErrTaskNotFound
	}

	return nil
}

// Delete -.
func (r *Repo) Delete(ctx context.Context, userID, taskID string) error {
	affected, err := r.q.DeleteTask(ctx, queries.DeleteTaskParams{ID: taskID, UserID: userID})
	if err != nil {
		return fmt.Errorf("TaskRepo - Delete - r.q.DeleteTask: %w", err)
	}

	if affected == 0 {
		return entity.ErrTaskNotFound
	}

	return nil
}
