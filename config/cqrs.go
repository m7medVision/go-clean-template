package config

import (
	"time"
)

// CQRS configures the event store and its projections: the app waits for the projections after a
// write, and the worker runs them.
type CQRS struct {
	// ConsistencyTimeout is how long a write waits for its projections before it answers that it is
	// saved but not yet readable.
	ConsistencyTimeout time.Duration `env:"CQRS_CONSISTENCY_TIMEOUT" envDefault:"2s"`
	// SnapshotEvery snapshots a stream each time it crosses a multiple of this many events.
	SnapshotEvery int `env:"EVENTSTORE_SNAPSHOT_EVERY" envDefault:"100"`
	// ProjectionPollInterval is how often an idle projection reads the store when no notification
	// arrives.
	ProjectionPollInterval time.Duration `env:"PROJECTION_POLL_INTERVAL" envDefault:"1s"`
	// ProjectionBatchSize is the most events a projection reads at once.
	ProjectionBatchSize int `env:"PROJECTION_BATCH_SIZE" envDefault:"100"`
}
