package config

// Analytics holds each analytics vendor's settings; a vendor is on when its key is set. Only the
// worker sends.
type Analytics struct {
	PostHog PostHog
}

type PostHog struct {
	APIKey string `env:"POSTHOG_API_KEY"`
	Host   string `env:"POSTHOG_HOST" envDefault:"https://eu.i.posthog.com"`
}
