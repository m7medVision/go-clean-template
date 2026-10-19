package smtp

import (
	"net/smtp"
	"time"
)

// Option -.
type Option func(*Client)

// Auth makes the client authenticate with PLAIN. An empty username leaves auth off, which is what a
// local relay such as Mailpit expects. net/smtp sends PLAIN credentials only over TLS or to localhost.
func Auth(username, password string) Option {
	return func(c *Client) {
		if username != "" {
			c.auth = smtp.PlainAuth("", username, password, c.host)
		}
	}
}

// Timeout bounds one whole send: dial, conversation and data.
func Timeout(d time.Duration) Option {
	return func(c *Client) {
		c.timeout = d
	}
}
