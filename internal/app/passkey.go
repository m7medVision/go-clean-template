package app

import (
	"github.com/m7medvision/go-clean-template/config"
	"github.com/m7medvision/go-clean-template/internal/repo/ceremony"
)

// newPasskeyCeremony builds the WebAuthn ceremony the passkey handlers share. It fails when the
// WebAuthn settings are invalid.
func newPasskeyCeremony(cfg *config.Config) (*ceremony.Ceremony, error) {
	return ceremony.New(cfg.JWT.Secret, &ceremony.Config{
		RPID:         cfg.WebAuthn.RPID,
		RPName:       cfg.WebAuthn.RPName,
		RPOrigins:    cfg.WebAuthn.RPOrigins,
		Timeout:      cfg.WebAuthn.CeremonyTimeout,
		ReauthMaxAge: cfg.WebAuthn.ReauthMaxAge,
	})
}
