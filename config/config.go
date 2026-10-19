package config

import (
	"fmt"
	"time"

	"github.com/caarlos0/env/v11"
)

type (
	// Config -.
	Config struct {
		App      app
		HTTP     http
		Log      log
		PG       pg
		GRPC     grpc
		JWT      jwt
		Metrics  metrics
		Docs     docs
		Tracing  tracing
		Worker   worker
		Mail     mail
		Exchange exchange
	}

	// App -.
	app struct {
		Name    string `env:"APP_NAME,required"`
		Version string `env:"APP_VERSION,required"`
		// BaseURL is where the web client is served; links in emails point under it.
		BaseURL string `env:"APP_BASE_URL" envDefault:"http://localhost"`
	}

	// HTTP -.
	http struct {
		Port string `env:"HTTP_PORT,required"`
	}

	// Log -.
	log struct {
		Level string `env:"LOG_LEVEL,required"`
	}

	// PG -.
	pg struct {
		PoolMax int    `env:"PG_POOL_MAX,required"`
		URL     string `env:"PG_URL,required"`
	}

	// GRPC -.
	grpc struct {
		Port string `env:"GRPC_PORT,required"`
	}

	// JWT -.
	jwt struct {
		Secret      string        `env:"JWT_SECRET,required"`
		TokenExpiry time.Duration `env:"JWT_TOKEN_EXPIRY" envDefault:"24h"`
	}

	// Metrics -.
	metrics struct {
		Enabled bool `env:"METRICS_ENABLED" envDefault:"true"`
	}

	// Docs -.
	docs struct {
		Enabled bool `env:"DOCS_ENABLED" envDefault:"false"`
	}

	// Tracing -.
	tracing struct {
		Enabled      bool    `env:"TRACING_ENABLED" envDefault:"false"`
		OTLPEndpoint string  `env:"TRACING_OTLP_ENDPOINT" envDefault:"localhost:4317"`
		OTLPInsecure bool    `env:"TRACING_OTLP_INSECURE" envDefault:"true"`
		SampleRate   float64 `env:"TRACING_SAMPLE_RATE" envDefault:"0.1"`
	}

	// Worker -.
	worker struct {
		PollInterval      time.Duration `env:"WORKER_POLL_INTERVAL" envDefault:"1s"`
		VisibilityTimeout time.Duration `env:"WORKER_VISIBILITY_TIMEOUT" envDefault:"30s"`
		BatchSize         int           `env:"WORKER_BATCH_SIZE" envDefault:"10"`
		ShutdownTimeout   time.Duration `env:"WORKER_SHUTDOWN_TIMEOUT" envDefault:"10s"`
	}

	// Mail -. Only the worker sends, so none of it is required.
	mail struct {
		SMTPHost     string `env:"SMTP_HOST" envDefault:"localhost"`
		SMTPPort     int    `env:"SMTP_PORT" envDefault:"1025"`
		SMTPUsername string `env:"SMTP_USERNAME"`
		SMTPPassword string `env:"SMTP_PASSWORD"`
		From         string `env:"MAIL_FROM" envDefault:"Go Clean Template <noreply@example.com>"`
	}

	// Exchange -.
	exchange struct {
		// ServiceURL is the base URL of the Frankfurter API that supplies exchange rates.
		// The integration tests point it at a local stub.
		ServiceURL string `env:"EXCHANGE_SERVICE_URL" envDefault:"https://api.frankfurter.dev"`
	}
)

// NewConfig returns app config.
func NewConfig() (*Config, error) {
	cfg := &Config{}
	if err := env.Parse(cfg); err != nil {
		return nil, fmt.Errorf("config error: %w", err)
	}

	return cfg, nil
}
