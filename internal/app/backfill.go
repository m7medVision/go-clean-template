package app

import (
	"github.com/m7medvision/go-clean-template/internal/repo"
	"github.com/m7medvision/go-clean-template/internal/repo/persistent/eventstore"
	persistExchangeRepo "github.com/m7medvision/go-clean-template/internal/repo/persistent/exchange"
	persistPasskeyRepo "github.com/m7medvision/go-clean-template/internal/repo/persistent/passkey"
	persistTaskRepo "github.com/m7medvision/go-clean-template/internal/repo/persistent/task"
	persistUserRepo "github.com/m7medvision/go-clean-template/internal/repo/persistent/user"
	"github.com/m7medvision/go-clean-template/pkg/postgres"
)

// importers lists, per domain, what turns the rows it wrote before event sourcing into events. Run
// calls them through eventstore.Backfill before it serves, and each runs once.
func importers(pg *postgres.Postgres, store repo.EventStore) []eventstore.Importer {
	return []eventstore.Importer{
		persistTaskRepo.NewImporter(pg, store),
		persistUserRepo.NewImporter(pg, store),
		persistExchangeRepo.NewImporter(pg, store),
		persistPasskeyRepo.NewImporter(pg, store),
	}
}
