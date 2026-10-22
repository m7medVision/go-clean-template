package config

import (
	"time"
)

// WebAuthn -. Passkeys only work on an origin listed in RPOrigins whose host is RPID or a
// subdomain of it. The defaults cover Caddy, a dev server and the app's own port.
type webAuthn struct {
	RPID      string   `env:"WEBAUTHN_RP_ID" envDefault:"localhost"`
	RPName    string   `env:"WEBAUTHN_RP_NAME" envDefault:"Go Clean Template"`
	RPOrigins []string `env:"WEBAUTHN_RP_ORIGINS" envDefault:"http://localhost,http://localhost:3000,http://localhost:8080" envSeparator:","`
	// CeremonyTimeout bounds a register or sign-in ceremony: the browser prompt's timeout and the
	// lifetime of the signed ceremony token.
	CeremonyTimeout time.Duration `env:"WEBAUTHN_CEREMONY_TIMEOUT" envDefault:"5m"`
	// ReauthMaxAge is how recently the caller must have signed in to add a passkey: the age of
	// their auth token. An older one is asked to sign in again.
	ReauthMaxAge time.Duration `env:"WEBAUTHN_REAUTH_MAX_AGE" envDefault:"5m"`
}
