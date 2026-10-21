package response

import (
	"encoding/json"

	"github.com/danielgtaylor/huma/v2"
	"github.com/m7medvision/go-clean-template/internal/entity"
)

// PasskeyCeremony starts a WebAuthn ceremony: the options to pass to the browser, and the token to
// send back with its answer.
type PasskeyCeremony struct {
	Options       PasskeyOptions `json:"options"`
	CeremonyToken string         `json:"ceremony_token" example:"eyJhbGciOiJIUzI1NiIs..."`
}

// PasskeyOptions is the WebAuthn options JSON (the publicKey member) as the use case wrote it.
type PasskeyOptions json.RawMessage

// MarshalJSON writes the options as they are.
func (o PasskeyOptions) MarshalJSON() ([]byte, error) {
	if len(o) == 0 {
		return []byte("null"), nil
	}

	return o, nil
}

// Schema describes the options as an open object.
func (PasskeyOptions) Schema(huma.Registry) *huma.Schema {
	return &huma.Schema{
		Type: huma.TypeObject,
		Description: "WebAuthn options JSON for PublicKeyCredential.parseCreationOptionsFromJSON " +
			"(or parseRequestOptionsFromJSON on sign-in).",
		AdditionalProperties: true,
	}
}

// PasskeyList -.
type PasskeyList struct {
	Passkeys []entity.Passkey `json:"passkeys" nullable:"false"`
}

// NewPasskeyList answers an empty list as [], not null.
func NewPasskeyList(passkeys []entity.Passkey) PasskeyList {
	if passkeys == nil {
		passkeys = []entity.Passkey{}
	}

	return PasskeyList{Passkeys: passkeys}
}
