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

	// scaffold:sso:begin
	// IdentityRepo links identities at an OpenID Connect provider, keyed by (issuer, subject), to
	// users. UserByIdentity of an unlinked identity is entity.ErrUserNotFound. CreateWithIdentity
	// stores a user with no password together with its identity; an email or username already taken
	// is entity.ErrAccountConflict, and an identity already linked is entity.ErrUserAlreadyExists.
	IdentityRepo interface {
		UserByIdentity(ctx context.Context, issuer, subject string) (entity.User, error)
		CreateWithIdentity(ctx context.Context, user *entity.User, issuer, subject string) error
	}
	// scaffold:sso:end

	// TaskRepo -.
	TaskRepo interface {
		Store(ctx context.Context, task *entity.Task) error
		GetByID(ctx context.Context, userID, taskID string) (entity.Task, error)
		List(ctx context.Context, userID string, filter TaskFilter) ([]entity.Task, int, error)
		// ListAll is List across every user.
		ListAll(ctx context.Context, filter TaskFilter) ([]entity.Task, int, error)
		Update(ctx context.Context, task *entity.Task) error
		Delete(ctx context.Context, userID, taskID string) error
	}

	// scaffold:passkeys:begin
	// PasskeyRepo stores users' WebAuthn credentials and the login challenges already used. A
	// credential ID already stored is entity.ErrPasskeyExists; an unknown one is
	// entity.ErrPasskeyNotFound, as is another user's passkey on Rename and Delete, which match on
	// the owner too. BurnChallenge records a challenge hash once: a second burn of the
	// same hash is entity.ErrPasskeyChallengeUsed.
	PasskeyRepo interface {
		Store(ctx context.Context, passkey *entity.Passkey) error
		ListByUser(ctx context.Context, userID string) ([]entity.Passkey, error)
		GetByID(ctx context.Context, id entity.PasskeyID) (entity.Passkey, error)
		UpdateUsage(ctx context.Context, id entity.PasskeyID, credential json.RawMessage, usedAt time.Time) error
		Rename(ctx context.Context, userID string, id entity.PasskeyID, name string) (entity.Passkey, error)
		Delete(ctx context.Context, userID string, id entity.PasskeyID) error
		BurnChallenge(ctx context.Context, hash []byte, expiresAt time.Time) error
		DeleteExpiredChallenges(ctx context.Context) error
	}
	// scaffold:passkeys:end

	// scaffold:queue:begin
	// QueueRepo is a Postgres-backed message queue. Use cases enqueue with Send; the worker binary
	// consumes with Read and Delete (it satisfies worker.Queue).
	QueueRepo interface {
		Send(ctx context.Context, queue string, body json.RawMessage) (int64, error)
		Read(ctx context.Context, queue string, vt time.Duration, qty int) ([]worker.Message, error)
		Delete(ctx context.Context, queue string, id int64) error
	}
	// scaffold:queue:end

	// scaffold:welcome:begin
	// UserNotifier tells a user about events on their account. The queue-backed implementation
	// (internal/repo/notify) enqueues an email for the worker to send.
	UserNotifier interface {
		Welcome(ctx context.Context, user *entity.User) error
	}
	// scaffold:welcome:end

	// scaffold:email:begin
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
	// scaffold:email:end

	// TaskFilter -.
	TaskFilter struct {
		Status *entity.TaskStatus
		Limit  int64
		Offset int64
	}
)
