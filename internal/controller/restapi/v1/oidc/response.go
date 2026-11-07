package oidc

type OIDCStatus struct { //nolint:revive // Huma names the OpenAPI schema after this type, so the name is API contract
	Enabled bool `json:"enabled" doc:"Whether SSO sign-in is configured"`
}

// OIDCRedirect is where to send the browser to sign in at the provider, and the ceremony state to
// send back to finish.
type OIDCRedirect struct { //nolint:revive // Huma names the OpenAPI schema after this type, so the name is API contract
	URL           string `json:"url"            doc:"The provider's authorize URL"`
	CeremonyToken string `json:"ceremony_token" doc:"Send it back to finish in the X-Ceremony-Token header"`
}
