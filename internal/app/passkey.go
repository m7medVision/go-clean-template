package app

import (
	"fmt"
	"time"

	restpasskey "github.com/m7medvision/go-clean-template/internal/controller/restapi/v1/passkey"
	"github.com/m7medvision/go-clean-template/internal/repo/ceremony"
	"github.com/m7medvision/go-clean-template/internal/repo/persistent/eventstore"
	persistPasskeyRepo "github.com/m7medvision/go-clean-template/internal/repo/persistent/passkey"
	persistUserRepo "github.com/m7medvision/go-clean-template/internal/repo/persistent/user"
	"github.com/m7medvision/go-clean-template/internal/repo/session"
	"github.com/m7medvision/go-clean-template/internal/usecase/passkey"
	"github.com/m7medvision/go-clean-template/pkg/cqrs"
)

// passkeyModule is the passkey domain: WebAuthn registration and sign-in, which reads the users'
// read model too, its importer and projector, and its REST operations.
func passkeyModule() module {
	return module{
		handlers: func(d *deps, b cqrs.Buses) error {
			// Invalid WebAuthn settings fail here, so they stop the app at startup.
			c, err := ceremony.New(d.cfg.JWT.Secret, d.cfg.WebAuthn, time.Now)
			if err != nil {
				return fmt.Errorf("passkey - ceremony.New: %w", err)
			}

			passkey.Register(b.Commands, b.Queries, &passkey.Deps{
				Ceremony: c, Store: d.store, Users: persistUserRepo.NewReadModel(d.pg),
				Passkeys: persistPasskeyRepo.NewReadModel(d.pg), Guard: persistPasskeyRepo.NewGuard(d.pg),
				Sessions: session.New(d.tokens),
			})

			return nil
		},
		importer:  func(d *deps) eventstore.Importer { return persistPasskeyRepo.NewImporter(d.pg) },
		rest:      restpasskey.NewRoutes,
		projector: persistPasskeyRepo.NewProjector,
	}
}
