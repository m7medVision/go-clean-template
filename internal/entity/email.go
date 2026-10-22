package entity

import (
	"encoding/json"
	"fmt"
	"reflect"
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

// EmailTypes lists one zero value of every Email type. DecodeEmail reads only these, so a new
// type needs an entry here; each type lives in its own email_<name>.go file.
func EmailTypes() []Email {
	return []Email{
		WelcomeEmail{}, // scaffold:welcome
	}
}

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

// DecodeEmail reads a queue body written by EncodeEmail. A template that is not in EmailTypes
// returns ErrUnknownEmail.
func DecodeEmail(body json.RawMessage) (Email, error) {
	var envelope emailEnvelope
	if err := json.Unmarshal(body, &envelope); err != nil {
		return nil, fmt.Errorf("DecodeEmail - json.Unmarshal envelope: %w", err)
	}

	for _, kind := range EmailTypes() {
		if kind.Template() != envelope.Template {
			continue
		}

		// A new value of kind's concrete type, filled from the data.
		e := reflect.New(reflect.TypeOf(kind))
		if err := json.Unmarshal(envelope.Data, e.Interface()); err != nil {
			return nil, fmt.Errorf("DecodeEmail - json.Unmarshal %s: %w", envelope.Template, err)
		}

		email, ok := e.Elem().Interface().(Email)
		if !ok {
			return nil, fmt.Errorf("DecodeEmail: %w: %q", ErrUnknownEmail, envelope.Template)
		}

		return email, nil
	}

	return nil, fmt.Errorf("DecodeEmail: %w: %q", ErrUnknownEmail, envelope.Template)
}
