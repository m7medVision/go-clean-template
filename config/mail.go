package config

// Mail has no required field: only the worker sends.
type Mail struct {
	SMTPHost     string `env:"SMTP_HOST" envDefault:"localhost"`
	SMTPPort     int    `env:"SMTP_PORT" envDefault:"1025"`
	SMTPUsername string `env:"SMTP_USERNAME"`
	SMTPPassword string `env:"SMTP_PASSWORD"`
	From         string `env:"MAIL_FROM" envDefault:"Go Clean Template <noreply@example.com>"`
}
