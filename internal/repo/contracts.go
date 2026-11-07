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

//go:generate mockgen -source=contracts.go -destination=repomock/repomock.go -package=repomock

type (
	// EventStore keeps aggregates as streams of events.
	//
	// Save appends a's pending events, and reserves or releases claims, in one transaction (it joins
	// the one the context carries), then marks a saved and returns the position of its last event.
	// A stream that moved on since a was loaded is entity.ErrConcurrencyConflict; a claim another
	// owner holds is that claim's Taken error. Saving with nothing pending writes nothing and
	// returns 0.
	//
	// Load replays the stream into a fresh a: its latest snapshot, then the events after it. A
	// stream with no events is entity.ErrStreamNotFound. ClaimOwner returns who holds a claim, or
	// entity.ErrClaimNotFound.
	EventStore interface {
		Save(ctx context.Context, a entity.Aggregate, claims ...entity.Claim) (int64, error)
		Load(ctx context.Context, streamID string, a entity.Aggregate) error
		ClaimOwner(ctx context.Context, scope, value string) (string, error)
	}

	// Subscriber is a projector or an event handler. The projection runner hands it every stored
	// event in order, inside the transaction that moves its checkpoint, so whatever it writes
	// through the context's transaction commits exactly once with the checkpoint.
	Subscriber interface {
		Name() string
		Handle(ctx context.Context, e *entity.RecordedEvent) error
	}

	// Importer turns the rows one domain wrote before it stored events into events, once (see
	// eventstore.Backfill). Import runs inside the backfill's transaction: it saves through the
	// event store, which joins it.
	Importer interface {
		Domain() string
		Import(ctx context.Context) error
	}

	// ProjectionCheckpoints tells how far the projectors have read the event store. Reached reports
	// whether every projector in names has handled the event at position.
	ProjectionCheckpoints interface {
		Reached(ctx context.Context, names []string, position int64) (bool, error)
	}

	// ExchangeReadModel reads exchange_history, which only the exchange projector writes. History is
	// one page of the user's conversions, newest first, and their total count. Conversion of
	// another user's or a missing conversion is entity.ErrConversionNotFound.
	ExchangeReadModel interface {
		History(ctx context.Context, userID string, page entity.Page) (entity.List[entity.Conversion], error)
		Conversion(ctx context.Context, userID, conversionID string) (entity.Conversion, error)
	}

	// ExchangeWebAPI fetches the reference rate from one currency to another, and the date it
	// was published. An unknown currency is entity.ErrUnsupportedCurrency.
	ExchangeWebAPI interface {
		Rate(ctx context.Context, from, to string) (decimal.Decimal, time.Time, error)
	}

	// SessionIssuer signs the auth token a sign-in answers with, carrying the user's roles.
	SessionIssuer interface {
		Issue(userID string, roles []entity.Role) (string, error)
	}

	// UserReadModel reads the users table, which only the user projector writes. A missing user is
	// entity.ErrUserNotFound.
	UserReadModel interface {
		ByID(ctx context.Context, id string) (entity.User, error)
		ByEmail(ctx context.Context, email string) (entity.User, error)
	}

	// TaskReadModel reads the tasks table, which only the task projector writes. Get of a missing
	// task is entity.ErrTaskNotFound, and of another user's task entity.ErrTaskForbidden.
	TaskReadModel interface {
		Get(ctx context.Context, userID, taskID string) (entity.Task, error)
		// List is one page of the user's tasks, of status when it is set, newest first.
		List(ctx context.Context, userID string, status *entity.TaskStatus, page entity.Page) (entity.List[entity.Task], error)
		// ListAll is List across every user.
		ListAll(ctx context.Context, status *entity.TaskStatus, page entity.Page) (entity.List[entity.Task], error)
	}

	// PasskeyReadModel reads webauthn_credentials, which only the passkey projector writes.
	// ListByUser is the user's passkeys, oldest first. Get of another user's or a missing passkey is
	// entity.ErrPasskeyNotFound.
	PasskeyReadModel interface {
		ListByUser(ctx context.Context, userID string) ([]entity.Passkey, error)
		Get(ctx context.Context, userID string, id entity.PasskeyID) (entity.Passkey, error)
	}

	// ChallengeGuard keeps the login challenges already used, a write-side guard a passkey login
	// writes in the same transaction as its events. BurnChallenge records a challenge hash once: a
	// second burn of the same hash is entity.ErrPasskeyChallengeUsed.
	ChallengeGuard interface {
		BurnChallenge(ctx context.Context, hash []byte, expiresAt time.Time) error
		DeleteExpiredChallenges(ctx context.Context) error
	}

	// Transactor runs fn in one transaction, carried by the context fn receives; every repository and
	// the event store join it. *postgres.Postgres implements it.
	Transactor interface {
		InTx(ctx context.Context, fn func(ctx context.Context) error) error
	}

	// QueueRepo is a Postgres-backed message queue. Use cases enqueue with Send; the worker binary
	// consumes with Read and Delete (it satisfies worker.Queue).
	QueueRepo interface {
		Send(ctx context.Context, queue string, body json.RawMessage) (int64, error)
		Read(ctx context.Context, queue string, vt time.Duration, qty int) ([]worker.Message, error)
		Delete(ctx context.Context, queue string, id int64) error
	}

	// UserNotifier tells a user about events on their account. The queue-backed implementation
	// (internal/repo/notify) enqueues an email for the worker to send.
	UserNotifier interface {
		Welcome(ctx context.Context, user *entity.User) error
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
)
