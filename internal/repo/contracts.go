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

	// SSOProvider runs a sign-in at the configured OpenID Connect provider. With none configured,
	// Enabled is false and Begin and Verify are entity.ErrOIDCDisabled. Begin returns the provider
	// URL to send the browser to and a token carrying the sign-in's state; Verify checks the
	// provider's redirect against it and returns who signed in. A bad, expired or replayed sign-in is
	// entity.ErrOIDCCeremonyInvalid, and an unreachable provider entity.ErrOIDCUnavailable.
	SSOProvider interface {
		Enabled() bool
		Begin(ctx context.Context) (authURL, token string, err error)
		Verify(ctx context.Context, code, state, token string) (entity.SSOIdentity, error)
	}

	// PasskeyCeremony runs the WebAuthn ceremonies of passkey registration and sign-in. Callers pass
	// and get the JSON the browser speaks; a ceremony's state travels from begin to finish in its
	// token. It stores nothing. Anything that does not complete a ceremony is
	// entity.ErrPasskeyCeremonyInvalid.
	PasskeyCeremony interface {
		// CheckReauth refuses a sign-in at authTime too old to add a passkey, with
		// entity.ErrPasskeyReauthRequired.
		CheckReauth(authTime time.Time) error
		// BeginRegistration starts adding a passkey for user, excluding the passkeys they have.
		BeginRegistration(user *entity.User, stored []entity.Passkey) (entity.CeremonyStarted, error)
		// FinishRegistration verifies the browser's new credential against the ceremony in token,
		// begun for userID, and returns its ID and the record to store.
		FinishRegistration(userID, token string, body []byte) (entity.PasskeyID, json.RawMessage, error)
		// BeginLogin starts a sign-in with any passkey the authenticator holds for the app.
		BeginLogin() (entity.CeremonyStarted, error)
		// LoginFor reads the user and passkey an assertion names, before anything is verified.
		LoginFor(body []byte) (userID string, id entity.PasskeyID, err error)
		// FinishLogin verifies the assertion in body against the ceremony in token, for user's
		// passkey as stored, and returns the sign-in to record. A sign count that did not advance
		// (a possible clone) does not complete it.
		FinishLogin(token string, body []byte, user *entity.User, passkey *entity.Passkey) (entity.PasskeyLogin, error)
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
