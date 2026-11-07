package config

import (
	"fmt"
	"strings"
	"time"

	"github.com/caarlos0/env/v11"
)

type (
	// Config is every setting, one section per concern. The binaries pass each part only the
	// sections it reads.
	Config struct {
		App      App
		HTTP     HTTP
		Log      Log
		PG       PG
		GRPC     GRPC
		JWT      JWT
		Metrics  Metrics
		Docs     Docs
		Tracing  Tracing
		Worker   Worker
		CQRS     CQRS
		Mail     Mail
		Exchange Exchange
		WebAuthn WebAuthn
		OIDC     OIDC
	}

	// App -.
	App struct {
		Name    string `env:"APP_NAME,required"`
		Version string `env:"APP_VERSION,required"`
		// BaseURL is the public origin users reach the app at, without a trailing slash; the links
		// the app builds point under it.
		BaseURL string `env:"APP_BASE_URL" envDefault:"http://localhost"`
	}

	// HTTP -.
	HTTP struct {
		Port string `env:"HTTP_PORT,required"`
	}

	// Log -.
	Log struct {
		Level string `env:"LOG_LEVEL,required"`
	}

	// PG -.
	PG struct {
		PoolMax int    `env:"PG_POOL_MAX,required"`
		URL     string `env:"PG_URL,required"`
	}

	// JWT -.
	JWT struct {
		Secret      string        `env:"JWT_SECRET,required"`
		TokenExpiry time.Duration `env:"JWT_TOKEN_EXPIRY" envDefault:"15m"`
	}

	// Metrics -.
	Metrics struct {
		Enabled bool `env:"METRICS_ENABLED" envDefault:"true"`
	}

	// Docs -.
	Docs struct {
		Enabled bool `env:"DOCS_ENABLED" envDefault:"false"`
	}

	// Tracing has the fields of tracing.Config, so it converts to one.
	Tracing struct {
		Enabled    bool    `env:"TRACING_ENABLED" envDefault:"false"`
		Endpoint   string  `env:"TRACING_OTLP_ENDPOINT" envDefault:"localhost:4317"`
		Insecure   bool    `env:"TRACING_OTLP_INSECURE" envDefault:"true"`
		SampleRate float64 `env:"TRACING_SAMPLE_RATE" envDefault:"0.1"`
	}

	// Exchange -.
	Exchange struct {
		// ServiceURL is the base URL of the ExchangeRate-API open access endpoint that supplies
		// exchange rates. The integration tests point it at a local stub.
		ServiceURL string `env:"EXCHANGE_SERVICE_URL" envDefault:"https://open.er-api.com"`
	}
)

// NewConfig returns app config.
func NewConfig() (*Config, error) {
	cfg := &Config{}
	if err := env.Parse(cfg); err != nil {
		return nil, fmt.Errorf("config error: %w", err)
	}

	cfg.App.BaseURL = strings.TrimSuffix(cfg.App.BaseURL, "/")

	return cfg, nil
}
