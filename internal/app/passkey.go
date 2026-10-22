package app

import (
	"fmt"

	"github.com/m7medvision/go-clean-template/config"
	"github.com/m7medvision/go-clean-template/internal/repo"
	persistPasskeyRepo "github.com/m7medvision/go-clean-template/internal/repo/persistent/passkey"
	persistUserRepo "github.com/m7medvision/go-clean-template/internal/repo/persistent/user"
	"github.com/m7medvision/go-clean-template/internal/usecase"
	"github.com/m7medvision/go-clean-template/internal/usecase/passkey"
	"github.com/m7medvision/go-clean-template/pkg/jwt"
)

// _webauthnCeremonyScope scopes the WebAuthn ceremony token's key and audience.
const _webauthnCeremonyScope = "webauthn-ceremony"

// addPasskeyUseCase adds the passkey use case. It fails when the WebAuthn settings are invalid.
func addPasskeyUseCase(deps featureDeps, uc *useCases) error {
	passkeyUseCase, err := newPasskeyUseCase(deps.cfg, persistUserRepo.New(deps.pg), persistPasskeyRepo.New(deps.pg), deps.jwt)
	if err != nil {
		return err
	}

	uc.passkey = passkeyUseCase

	return nil
}

func newPasskeyUseCase(
	cfg *config.Config, users repo.UserRepo, passkeys repo.PasskeyRepo, tokens *jwt.Manager,
) (usecase.Passkey, error) {
	ceremony, err := jwt.NewScoped(cfg.JWT.Secret, _webauthnCeremonyScope, cfg.WebAuthn.CeremonyTimeout)
	if err != nil {
		return nil, fmt.Errorf("jwt.NewScoped: %w", err)
	}

	uc, err := passkey.New(users, passkeys, ceremony, tokens, &passkey.Config{
		RPID:         cfg.WebAuthn.RPID,
		RPName:       cfg.WebAuthn.RPName,
		RPOrigins:    cfg.WebAuthn.RPOrigins,
		Timeout:      cfg.WebAuthn.CeremonyTimeout,
		ReauthMaxAge: cfg.WebAuthn.ReauthMaxAge,
	})
	if err != nil {
		return nil, fmt.Errorf("passkey.New: %w", err)
	}

	return uc, nil
}
