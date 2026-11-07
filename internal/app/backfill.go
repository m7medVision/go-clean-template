package app

import (
	"github.com/m7medvision/go-clean-template/internal/repo"
)

// importers lists each module's importer: what turns the rows its domain wrote before event
// sourcing into events. Run calls them through eventstore.Backfill before it serves, and each runs
// once.
func importers(d *deps) []repo.Importer {
	var imps []repo.Importer

	for _, m := range modules() {
		if m.importer != nil {
			imps = append(imps, m.importer(d))
		}
	}

	return imps
}
