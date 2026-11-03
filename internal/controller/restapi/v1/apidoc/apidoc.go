// Package apidoc holds what the v1 operations of several domains share in the OpenAPI document: the
// bearer-token security scheme and the tag of the sign-in operations.
package apidoc

const (
	// BearerAuthScheme names the security scheme of every operation that needs a token.
	BearerAuthScheme = "BearerAuth"

	// AuthTag groups the sign-in operations: password, passkey and SSO.
	AuthTag = "auth"
)
