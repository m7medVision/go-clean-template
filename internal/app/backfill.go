package app

import (
	"github.com/m7medvision/go-clean-template/internal/repo/persistent/eventstore"
	"github.com/m7medvision/go-clean-template/pkg/postgres"
)

// importers lists each module's importer: what turns the rows its domain wrote before event
// sourcing into events. Run calls them through eventstore.Backfill before it serves, and each runs
// once; a rebuild refuses until they have.
func importers(pg *postgres.Postgres) []eventstore.Importer {
	var imps []eventstore.Importer

	for _, m := range modules() {
		if m.importer != nil {
			imps = append(imps, m.importer(pg))
		}
	}

	return imps
}
