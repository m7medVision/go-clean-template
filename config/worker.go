package config

import (
	"time"
)

// Worker -.
type worker struct {
	PollInterval      time.Duration `env:"WORKER_POLL_INTERVAL" envDefault:"1s"`
	VisibilityTimeout time.Duration `env:"WORKER_VISIBILITY_TIMEOUT" envDefault:"30s"`
	BatchSize         int           `env:"WORKER_BATCH_SIZE" envDefault:"10"`
	ShutdownTimeout   time.Duration `env:"WORKER_SHUTDOWN_TIMEOUT" envDefault:"10s"`
}
