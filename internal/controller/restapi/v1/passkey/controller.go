// Package passkey serves the v1 passkey sign-in and passkey management operations over REST.
package passkey

import (
	"context"
	"log/slog"
	"time"

	"github.com/danielgtaylor/huma/v2"
	"github.com/m7medvision/go-clean-template/internal/entity"
	"github.com/m7medvision/go-clean-template/internal/usecase/passkey/ceremony"
	"github.com/m7medvision/go-clean-template/internal/usecase/passkey/command"
	"github.com/m7medvision/go-clean-template/internal/usecase/passkey/query"
	"github.com/m7medvision/go-clean-template/pkg/cqrs"
)

const _passkeysTag = "passkeys"

type controller struct {
	cmds *cqrs.Commands
	qs   *cqrs.Queries
	l    *slog.Logger
}

// NewRoutes registers the passkey sign-in and passkey management operations.
func NewRoutes(api huma.API, b cqrs.Buses, l *slog.Logger) {
	r := &controller{cmds: b.Commands, qs: b.Queries, l: l}

	r.beginPasskeyLogin(api)
	r.finishPasskeyLogin(api)

	r.beginPasskeyRegistration(api)
	r.finishPasskeyRegistration(api)
	r.listPasskeys(api)
	r.renamePasskey(api)
	r.deletePasskey(api)
}

func (r *controller) beginRegistration(ctx context.Context, userID string, authTime time.Time) (ceremony.Started, error) {
	return cqrs.SendQuery[ceremony.Started](ctx, r.qs, query.BeginPasskeyRegistration{UserID: userID, AuthTime: authTime})
}

// finishRegistration registers the passkey, then reads it back for the response.
func (r *controller) finishRegistration(ctx context.Context, userID, token, name string, body []byte) (entity.Passkey, error) {
	written, err := cqrs.SendCommand[cqrs.Written](ctx, r.cmds,
		command.FinishPasskeyRegistration{UserID: userID, Token: token, Name: name, Body: body})
	if err != nil {
		return entity.Passkey{}, err
	}

	return cqrs.SendQuery[entity.Passkey](ctx, r.qs, query.GetPasskey{UserID: userID, ID: written.ID})
}

func (r *controller) list(ctx context.Context, userID string) ([]entity.Passkey, error) {
	return cqrs.SendQuery[[]entity.Passkey](ctx, r.qs, query.ListPasskeys{UserID: userID})
}

// rename renames the passkey, then reads it back for the response.
func (r *controller) rename(ctx context.Context, userID, id, name string) (entity.Passkey, error) {
	written, err := cqrs.SendCommand[cqrs.Written](ctx, r.cmds, command.RenamePasskey{UserID: userID, ID: id, Name: name})
	if err != nil {
		return entity.Passkey{}, err
	}

	return cqrs.SendQuery[entity.Passkey](ctx, r.qs, query.GetPasskey{UserID: userID, ID: written.ID})
}

func (r *controller) delete(ctx context.Context, userID, id string) error {
	_, err := cqrs.SendCommand[cqrs.Written](ctx, r.cmds, command.DeletePasskey{UserID: userID, ID: id})

	return err
}

func (r *controller) beginLogin(ctx context.Context) (ceremony.Started, error) {
	return cqrs.SendQuery[ceremony.Started](ctx, r.qs, query.BeginPasskeyLogin{})
}

func (r *controller) finishLogin(ctx context.Context, token string, body []byte) (string, error) {
	signedIn, err := cqrs.SendCommand[command.SignedIn](ctx, r.cmds, command.FinishPasskeyLogin{Token: token, Body: body})

	return signedIn.Token, err
}
