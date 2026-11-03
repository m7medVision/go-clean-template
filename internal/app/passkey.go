package app

import (
	"fmt"

	"github.com/m7medvision/go-clean-template/config"
	"github.com/m7medvision/go-clean-template/internal/usecase/passkey/ceremony"
	"github.com/m7medvision/go-clean-template/pkg/jwt"
)

// _webauthnCeremonyScope scopes the WebAuthn ceremony token's key and audience.
const _webauthnCeremonyScope = "webauthn-ceremony"

// newPasskeyCeremony builds the WebAuthn ceremony the passkey handlers share. It fails when the
// WebAuthn settings are invalid.
func newPasskeyCeremony(cfg *config.Config) (*ceremony.Ceremony, error) {
	signer, err := jwt.NewScoped(cfg.JWT.Secret, _webauthnCeremonyScope, cfg.WebAuthn.CeremonyTimeout)
	if err != nil {
		return nil, fmt.Errorf("jwt.NewScoped: %w", err)
	}

	c, err := ceremony.New(signer, &ceremony.Config{
		RPID:         cfg.WebAuthn.RPID,
		RPName:       cfg.WebAuthn.RPName,
		RPOrigins:    cfg.WebAuthn.RPOrigins,
		Timeout:      cfg.WebAuthn.CeremonyTimeout,
		ReauthMaxAge: cfg.WebAuthn.ReauthMaxAge,
	})
	if err != nil {
		return nil, fmt.Errorf("ceremony.New: %w", err)
	}

	return c, nil
}
