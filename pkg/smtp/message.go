// Package smtp sends email over SMTP.
package smtp

// Message is one rendered email. The client sets From.
type Message struct {
	To      string
	Subject string
	HTML    string
	Text    string
}
