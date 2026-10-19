package entity

import (
	"encoding/json"
	"fmt"
)

// EmailQueue is the queue use cases send email to and the worker delivers it from.
const EmailQueue = "mail"

// Email is one transactional email: the data its template renders. Template names the
// templates in internal/repo/mailtmpl/templates/ (<name>.html.tmpl and <name>.txt.tmpl), and every
// field they read must exist on the type, which the render test checks.
type Email interface {
	Template() string
	Recipient() string
}

// WelcomeEmail is sent once a user registers.
type WelcomeEmail struct {
	To       string `json:"to"`
	Username string `json:"username"`
	LoginURL string `json:"login_url"`
}

// Template -.
func (WelcomeEmail) Template() string { return "welcome" }

// Recipient -.
func (e WelcomeEmail) Recipient() string { return e.To }

// emailEnvelope is the queue body: the template name tells DecodeEmail which type data holds.
type emailEnvelope struct {
	Template string          `json:"template"`
	Data     json.RawMessage `json:"data"`
}

// EncodeEmail returns the queue body for e.
func EncodeEmail(e Email) (json.RawMessage, error) {
	data, err := json.Marshal(e)
	if err != nil {
		return nil, fmt.Errorf("EncodeEmail - json.Marshal: %w", err)
	}

	body, err := json.Marshal(emailEnvelope{Template: e.Template(), Data: data})
	if err != nil {
		return nil, fmt.Errorf("EncodeEmail - json.Marshal envelope: %w", err)
	}

	return body, nil
}

// DecodeEmail reads a queue body written by EncodeEmail. A template it does not know returns
// ErrUnknownEmail; a new Email type needs a case here.
func DecodeEmail(body json.RawMessage) (Email, error) {
	var envelope emailEnvelope
	if err := json.Unmarshal(body, &envelope); err != nil {
		return nil, fmt.Errorf("DecodeEmail - json.Unmarshal envelope: %w", err)
	}

	switch envelope.Template {
	case WelcomeEmail{}.Template():
		return decodeEmailData[WelcomeEmail](envelope.Data)
	default:
		return nil, fmt.Errorf("DecodeEmail: %w: %q", ErrUnknownEmail, envelope.Template)
	}
}

func decodeEmailData[T Email](data json.RawMessage) (Email, error) {
	var e T
	if err := json.Unmarshal(data, &e); err != nil {
		return nil, fmt.Errorf("DecodeEmail - json.Unmarshal %s: %w", e.Template(), err)
	}

	return e, nil
}
