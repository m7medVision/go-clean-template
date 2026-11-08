package config

import (
	"time"
)

// Worker has the fields of broker.Settings, so it converts to one.
type Worker struct {
	MaxRetries      int           `env:"WORKER_MAX_RETRIES" envDefault:"5"`
	RetryInitial    time.Duration `env:"WORKER_RETRY_INITIAL" envDefault:"1s"`
	RetryMax        time.Duration `env:"WORKER_RETRY_MAX" envDefault:"30s"`
	AckWait         time.Duration `env:"WORKER_ACK_WAIT" envDefault:"5m"`
	ShutdownTimeout time.Duration `env:"WORKER_SHUTDOWN_TIMEOUT" envDefault:"10s"`
}
