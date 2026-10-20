// Package repo implements application outer layer logic. Each logic group in own file.
package repo

import (
	"context"
	"encoding/json"
	"time"

	"github.com/m7medvision/go-clean-template/internal/entity"
	"github.com/m7medvision/go-clean-template/pkg/smtp"
	"github.com/m7medvision/go-clean-template/pkg/worker"
	"github.com/shopspring/decimal"
)

// The SMTP client is the Mailer the app wires in.
var _ Mailer = (*smtp.Client)(nil)

//go:generate mockgen -source=contracts.go -destination=../usecase/mocks_repo_test.go -package=usecase_test

type (
	// ExchangeRepo stores each user's currency conversions.
	ExchangeRepo interface {
		Store(ctx context.Context, userID string, c *entity.Conversion) error
		GetHistory(ctx context.Context, userID string, limit, offset int64) ([]entity.Conversion, int, error)
	}

	// ExchangeWebAPI fetches the reference rate from one currency to another, and the date it
	// was published. An unknown currency is entity.ErrUnsupportedCurrency.
	ExchangeWebAPI interface {
		Rate(ctx context.Context, from, to string) (decimal.Decimal, time.Time, error)
	}

	// UserRepo -.
	UserRepo interface {
		Store(ctx context.Context, user *entity.User) error
		GetByID(ctx context.Context, id string) (entity.User, error)
		GetByEmail(ctx context.Context, email string) (entity.User, error)
	}

	// TaskRepo -.
	TaskRepo interface {
		Store(ctx context.Context, task *entity.Task) error
		GetByID(ctx context.Context, userID, taskID string) (entity.Task, error)
		List(ctx context.Context, userID string, filter TaskFilter) ([]entity.Task, int, error)
		Update(ctx context.Context, task *entity.Task) error
		Delete(ctx context.Context, userID, taskID string) error
	}

	// QueueRepo is a Postgres-backed message queue. Use cases enqueue with Send; the worker binary
	// consumes with Read and Delete (it satisfies worker.Queue).
	QueueRepo interface {
		Send(ctx context.Context, queue string, body json.RawMessage) (int64, error)
		Read(ctx context.Context, queue string, vt time.Duration, qty int) ([]worker.Message, error)
		Delete(ctx context.Context, queue string, id int64) error
	}

	// MailRenderer turns an email into the message the mailer sends, from the templates embedded in
	// the binary.
	MailRenderer interface {
		Render(ctx context.Context, email entity.Email) (smtp.Message, error)
	}

	// Mailer delivers a rendered message. *smtp.Client implements it; another provider is another
	// implementation.
	Mailer interface {
		Send(ctx context.Context, m smtp.Message) error
	}

	// TaskFilter -.
	TaskFilter struct {
		Status *entity.TaskStatus
		Limit  int64
		Offset int64
	}
)
