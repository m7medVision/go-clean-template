package passkey

import (
	"context"
	"fmt"

	"github.com/m7medvision/go-clean-template/internal/entity"
	"github.com/m7medvision/go-clean-template/internal/repo"
	"github.com/m7medvision/go-clean-template/internal/repo/persistent/queries"
	"github.com/m7medvision/go-clean-template/pkg/postgres"
)

// Projector keeps webauthn_credentials, the passkey read model, up to date from the passkey events.
// The worker runs it; it writes through the transaction the runner's context carries.
type Projector struct {
	pg *postgres.Postgres
}

var _ repo.Subscriber = (*Projector)(nil)

// NewProjector returns the passkey projector writing to pg.
func NewProjector(pg *postgres.Postgres) *Projector {
	return &Projector{pg: pg}
}

// Name -.
func (*Projector) Name() string {
	return "passkey"
}

// Tables are what a rebuild empties.
func (*Projector) Tables() []string {
	return []string{"webauthn_credentials"}
}

// Handle -.
func (p *Projector) Handle(ctx context.Context, e *entity.RecordedEvent) error {
	return Project(ctx, queries.New(p.pg.Executor(ctx)), e)
}

// Project applies one event to webauthn_credentials through q. Events of other domains change
// nothing.
func Project(ctx context.Context, q queries.Querier, e *entity.RecordedEvent) error {
	var err error

	switch ev := e.Event.(type) {
	case entity.PasskeyRegistered:
		err = q.InsertPasskeyRow(ctx, queries.InsertPasskeyRowParams{
			ID: ev.PasskeyID, UserID: ev.UserID, Name: ev.Name, Credential: ev.Credential, CreatedAt: ev.At,
		})
	case entity.PasskeyImported:
		err = q.InsertPasskeyRow(ctx, queries.InsertPasskeyRowParams{
			ID: ev.PasskeyID, UserID: ev.UserID, Name: ev.Name, Credential: ev.Credential,
			CreatedAt: ev.CreatedAt, LastUsedAt: ev.LastUsedAt,
		})
	case entity.PasskeyRenamed:
		err = q.SetPasskeyName(ctx, queries.SetPasskeyNameParams{ID: ev.PasskeyID, Name: ev.Name})
	case entity.PasskeyUsed:
		at := ev.At
		err = q.SetPasskeyUsage(ctx, queries.SetPasskeyUsageParams{ID: ev.PasskeyID, Credential: ev.Credential, LastUsedAt: &at})
	case entity.PasskeyDeleted:
		err = q.DeletePasskeyRow(ctx, ev.PasskeyID)
	}

	if err != nil {
		return fmt.Errorf("PasskeyProjector - %s at %d: %w", e.Event.EventType(), e.Position, err)
	}

	return nil
}
