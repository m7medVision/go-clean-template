// Package passkey serves the v1 passkey sign-in and passkey management operations over REST.
package passkey

import (
	"log/slog"

	"github.com/danielgtaylor/huma/v2"
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
