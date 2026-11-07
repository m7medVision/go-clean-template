// Package ceremonytest plays the browser and the authenticator for the passkey tests: it answers
// creation and request options with attestations and assertions from a virtual authenticator, as
// navigator.credentials.create/get and PublicKeyCredential.toJSON would.
package ceremonytest

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/descope/virtualwebauthn"
	"github.com/go-webauthn/webauthn/webauthn"
	"github.com/stretchr/testify/require"
)

// The relying party the tests register with, and the ceremony's settings.
const (
	RPID            = "localhost"
	RPName          = "Go Clean Template"
	RPOrigin        = "http://localhost:8080"
	CeremonyPurpose = "webauthn-ceremony"
	CeremonyTimeout = 5 * time.Minute
	ReauthMaxAge    = 5 * time.Minute
	SigningKey      = "test-secret"
)

func relyingParty() virtualwebauthn.RelyingParty {
	return virtualwebauthn.RelyingParty{ID: RPID, Name: RPName, Origin: RPOrigin}
}

// Attest answers creation options with an attestation for cred from a fresh authenticator.
func Attest(t *testing.T, options []byte, cred virtualwebauthn.Credential) []byte {
	t.Helper()

	auth := virtualwebauthn.NewAuthenticator()

	return AttestWith(t, options, cred, &auth)
}

// AttestWith is Attest from a given authenticator.
func AttestWith(t *testing.T, options []byte, cred virtualwebauthn.Credential, auth *virtualwebauthn.Authenticator) []byte {
	t.Helper()

	parsed, err := virtualwebauthn.ParseAttestationOptions(string(options))
	require.NoError(t, err)

	return []byte(virtualwebauthn.CreateAttestationResponse(relyingParty(), *auth, cred, *parsed))
}

// Assert answers discoverable request options with an assertion for cred from auth.
func Assert(t *testing.T, options []byte, cred virtualwebauthn.Credential, auth *virtualwebauthn.Authenticator) []byte {
	t.Helper()

	parsed, err := virtualwebauthn.ParseAssertionOptions(string(options))
	require.NoError(t, err)
	require.Empty(t, parsed.AllowCredentials, "a discoverable login names no credentials")

	return []byte(virtualwebauthn.CreateAssertionResponse(relyingParty(), *auth, cred, *parsed))
}

// Tamper flips one byte of a token's payload segment, keeping it valid base64url.
func Tamper(t *testing.T, token string) string {
	t.Helper()

	const halves = 2

	b := []byte(token)
	i := len(b) / halves

	require.NotEqual(t, byte('.'), b[i])

	if b[i] == 'A' {
		b[i] = 'B'
	} else {
		b[i] = 'A'
	}

	return string(b)
}

// WithSignCount returns a stored credential record with its sign count set to n.
func WithSignCount(t *testing.T, record json.RawMessage, n uint32) json.RawMessage {
	t.Helper()

	var credential webauthn.Credential

	require.NoError(t, json.Unmarshal(record, &credential))

	credential.Authenticator.SignCount = n

	out, err := json.Marshal(credential)
	require.NoError(t, err)

	return out
}
