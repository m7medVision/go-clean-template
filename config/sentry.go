package config

// Sentry is read by both binaries: each reports to its own GlitchTip project, so compose gives each
// its own SENTRY_DSN. Empty is off.
type Sentry struct {
	DSN         string `env:"SENTRY_DSN"`
	Environment string `env:"SENTRY_ENVIRONMENT" envDefault:"development"`
}
