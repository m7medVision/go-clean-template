package passkey

import (
	"context"

	"github.com/m7medvision/go-clean-template/internal/entity"
	"github.com/m7medvision/go-clean-template/internal/repo/persistent/projection"
	"github.com/m7medvision/go-clean-template/internal/repo/persistent/queries"
	"github.com/m7medvision/go-clean-template/pkg/postgres"
)

// NewProjector keeps webauthn_credentials, the passkey read model, up to date from the passkey
// events. The worker runs it.
func NewProjector(pg *postgres.Postgres) projection.Projector {
	return projection.NewProjector("passkey", []string{"webauthn_credentials"}, entity.PasskeyEvents(), pg, project)
}

// project applies one event to webauthn_credentials through q.
func project(ctx context.Context, q queries.Querier, e *entity.RecordedEvent) error {
	switch ev := e.Event.(type) {
	case entity.PasskeyRegistered:
		return q.InsertPasskeyRow(ctx, queries.InsertPasskeyRowParams{
			ID: ev.PasskeyID, UserID: ev.UserID, Name: ev.Name, Credential: ev.Credential, CreatedAt: ev.At,
		})
	case entity.PasskeyImported:
		return q.InsertPasskeyRow(ctx, queries.InsertPasskeyRowParams{
			ID: ev.PasskeyID, UserID: ev.UserID, Name: ev.Name, Credential: ev.Credential,
			CreatedAt: ev.CreatedAt, LastUsedAt: ev.LastUsedAt,
		})
	case entity.PasskeyRenamed:
		return q.SetPasskeyName(ctx, queries.SetPasskeyNameParams{ID: ev.PasskeyID, Name: ev.Name})
	case entity.PasskeyUsed:
		at := ev.At

		return q.SetPasskeyUsage(ctx, queries.SetPasskeyUsageParams{ID: ev.PasskeyID, Credential: ev.Credential, LastUsedAt: &at})
	case entity.PasskeyDeleted:
		return q.DeletePasskeyRow(ctx, ev.PasskeyID)
	default:
		return entity.ErrUnhandledEvent
	}
}
