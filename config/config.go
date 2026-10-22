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
		GRPC     grpc // scaffold:grpc
		JWT      jwt
		Metrics  metrics
		Docs     docs
		Tracing  tracing
		Worker   worker // scaffold:queue
		Mail     mail   // scaffold:email
		Exchange exchange
		WebAuthn webAuthn // scaffold:passkeys
		OIDC     oidc     // scaffold:sso
	}

	// App -.
	app struct {
		Name    string `env:"APP_NAME,required"`
		Version string `env:"APP_VERSION,required"`
		// BaseURL is the public origin users reach the app at; the links the app builds point under it.
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

	// JWT -.
	jwt struct {
		Secret      string        `env:"JWT_SECRET,required"`
		TokenExpiry time.Duration `env:"JWT_TOKEN_EXPIRY" envDefault:"15m"`
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

	// Exchange -.
	exchange struct {
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

	return cfg, nil
}
