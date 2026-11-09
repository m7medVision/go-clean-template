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
		App       App
		HTTP      HTTP
		Log       Log
		PG        PG
		GRPC      GRPC
		JWT       JWT
		Valkey    Valkey
		Mongo     Mongo
		NATS      NATS
		AuthLimit AuthLimit
		Metrics   Metrics
		Docs      Docs
		Tracing   Tracing
		Worker    Worker
		CQRS      CQRS
		Mail      Mail
		Analytics Analytics
		Exchange  Exchange
		WebAuthn  WebAuthn
		OIDC      OIDC
		Sentry    Sentry
	}

	App struct {
		Name    string `env:"APP_NAME,required"`
		Version string `env:"APP_VERSION,required"`
		// BaseURL is the public origin users reach the app at, without a trailing slash; the links
		// the app builds point under it.
		BaseURL string `env:"APP_BASE_URL" envDefault:"http://localhost"`
	}

	// HTTP is read only by the app; the worker serves no HTTP.
	HTTP struct {
		Port         string `env:"HTTP_PORT" envDefault:"8080"`
		TrustedProxy string `env:"TRUSTED_PROXY"`
	}

	Log struct {
		Level string `env:"LOG_LEVEL,required"`
	}

	PG struct {
		PoolMax int    `env:"PG_POOL_MAX,required"`
		URL     string `env:"PG_URL,required"`
	}

	// JWT is read only by the app, which refuses to start without a secret; the worker issues and
	// checks no token.
	JWT struct {
		Secret      string        `env:"JWT_SECRET"`
		TokenExpiry time.Duration `env:"JWT_TOKEN_EXPIRY" envDefault:"15m"`
	}

	// Valkey has no required tag: the worker parses the same config and has no Valkey, so app.Run
	// checks it.
	Valkey struct {
		URL string `env:"VALKEY_URL"`
	}

	// Mongo has no required tag: the worker parses the same config and has no Mongo, so app.Run
	// checks it. The database is the URL's path.
	Mongo struct {
		URL string `env:"MONGO_URL"`
	}

	// NATS has no required tag: the app parses the same config and never uses it, so RunWorker
	// checks it.
	NATS struct {
		URL string `env:"NATS_URL"`
	}

	AuthLimit struct {
		ClientAttempts  int           `env:"AUTH_LIMIT_CLIENT_ATTEMPTS" envDefault:"20"`
		AccountFailures int           `env:"AUTH_LIMIT_ACCOUNT_FAILURES" envDefault:"5"`
		Window          time.Duration `env:"AUTH_LIMIT_WINDOW" envDefault:"15m"`
	}

	Metrics struct {
		Enabled bool `env:"METRICS_ENABLED" envDefault:"true"`
	}

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

	Exchange struct {
		// ServiceURL is the base URL of the ExchangeRate-API open access endpoint that supplies
		// exchange rates. The integration tests point it at a local stub.
		ServiceURL string `env:"EXCHANGE_SERVICE_URL" envDefault:"https://open.er-api.com"`
	}
)

// The web client's routes the app links to. web/src/routes/ defines them; this is the one place the
// Go code names them.
const (
	_loginPath       = "/login"
	_ssoCallbackPath = "/sso/callback"
)

// LoginURL is the web client's sign-in page, which the welcome email links to.
func (a App) LoginURL() string {
	return a.BaseURL + _loginPath
}

// SSOCallbackURL is the web client's route the OIDC provider sends the browser back to; it must be a
// redirect URI of the provider's client.
func (a App) SSOCallbackURL() string {
	return a.BaseURL + _ssoCallbackPath
}

func NewConfig() (*Config, error) {
	cfg := &Config{}
	if err := env.Parse(cfg); err != nil {
		return nil, fmt.Errorf("config error: %w", err)
	}

	cfg.App.BaseURL = strings.TrimSuffix(cfg.App.BaseURL, "/")

	return cfg, nil
}
