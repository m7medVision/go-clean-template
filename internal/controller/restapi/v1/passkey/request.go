package passkey

import (
	"encoding/json"

	"github.com/danielgtaylor/huma/v2"
)

// PasskeyCredential is the browser's PublicKeyCredential.toJSON() as sent, unparsed. Its shape is
// owned by the WebAuthn spec and checked by the use case, so the schema only requires an object.
type PasskeyCredential json.RawMessage //nolint:revive // Huma names the OpenAPI schema after this type, so the name is API contract

func (c *PasskeyCredential) UnmarshalJSON(data []byte) error {
	*c = append((*c)[:0], data...)

	return nil
}

func (PasskeyCredential) Schema(huma.Registry) *huma.Schema {
	return &huma.Schema{
		Type: huma.TypeObject,
		Description: "The browser's PublicKeyCredential.toJSON() from navigator.credentials.create " +
			"(or navigator.credentials.get on sign-in).",
		AdditionalProperties: true,
	}
}

// RenamePasskey's name is trimmed by the use case; one that is blank after trimming gets the default
// name.
type RenamePasskey struct {
	_ struct{} `additionalProperties:"true"`

	Name string `json:"name" minLength:"1" maxLength:"64" example:"Work laptop"`
}
