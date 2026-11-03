// Package passkey serves the v1 passkey sign-in and passkey management operations over REST.
package passkey

import (
	"github.com/danielgtaylor/huma/v2"
	"github.com/m7medvision/go-clean-template/internal/usecase"
	"github.com/m7medvision/go-clean-template/pkg/logger"
)

const _passkeysTag = "passkeys"

type controller struct {
	pk usecase.Passkey
	l  logger.Interface
}

// NewRoutes registers the passkey sign-in and passkey management operations.
func NewRoutes(api huma.API, pk usecase.Passkey, l logger.Interface) {
	r := &controller{pk: pk, l: l}

	r.beginPasskeyLogin(api)
	r.finishPasskeyLogin(api)

	r.beginPasskeyRegistration(api)
	r.finishPasskeyRegistration(api)
	r.listPasskeys(api)
	r.renamePasskey(api)
	r.deletePasskey(api)
}
