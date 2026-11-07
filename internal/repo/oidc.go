package repo

import (
	"context"

	"github.com/m7medvision/go-clean-template/internal/entity"
)

// SSOProvider runs a sign-in at the configured OpenID Connect provider. With none configured,
// Enabled is false and Begin and Verify are entity.ErrOIDCDisabled. Begin returns the provider
// URL to send the browser to and a token carrying the sign-in's state; Verify checks the
// provider's redirect against it and returns who signed in. A bad, expired or replayed sign-in is
// entity.ErrOIDCCeremonyInvalid. A provider that fails at any step (unreachable, timed out, an error
// status, keys that cannot be fetched) is entity.ErrOIDCUnavailable; a canceled ctx is neither.
type SSOProvider interface {
	Enabled() bool
	Begin(ctx context.Context) (authURL, token string, err error)
	Verify(ctx context.Context, code, state, token string) (entity.SSOIdentity, error)
}
