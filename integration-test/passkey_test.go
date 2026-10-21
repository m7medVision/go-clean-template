package integration_test

import (
	"bytes"
	"context"
	"encoding/base64"
	"io"
	"net/http"
	"net/url"
	"testing"

	"github.com/descope/virtualwebauthn"
	"github.com/goccy/go-json"
)

// The relying party the test's virtual browser declares. Compose's app must list this origin in
// WEBAUTHN_RP_ORIGINS and use this RP ID.
const (
	rpID     = "localhost"
	rpName   = "Go Clean Template"
	rpOrigin = "http://localhost:8080"
)

// passkeyLoginFinish is the endpoint that completes a passkey sign-in.
const passkeyLoginFinish = basePathV1 + "/auth/passkey/finish"

type passkeyCeremony struct {
	Options       json.RawMessage `json:"options"`
	CeremonyToken string          `json:"ceremony_token"`
}

// ceremonyHeader carries the ceremony token to both finish endpoints. It is a header, not a query
// parameter, so the token stays out of access logs.
const ceremonyHeader = "X-Ceremony-Token"

// postPasskey sends a POST with an optional bearer token and ceremony token, and returns the status
// and body.
func postPasskey(t *testing.T, target, token, ceremony string, body []byte) (status int, respBody []byte) {
	t.Helper()

	return sendPasskey(t, http.MethodPost, target, token, ceremony, body)
}

// sendPasskey sends a request with an optional bearer token and ceremony token, and returns the
// status and body.
func sendPasskey(t *testing.T, method, target, token, ceremony string, body []byte) (status int, respBody []byte) {
	t.Helper()

	ctx, cancel := context.WithTimeout(t.Context(), requestTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, method, target, bytes.NewReader(body))
	if err != nil {
		t.Fatalf("%s %s: %v", method, target, err)
	}

	req.Header.Set("Content-Type", "application/json")

	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	if ceremony != "" {
		req.Header.Set(ceremonyHeader, ceremony)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("%s %s: %v", method, target, err)
	}

	defer resp.Body.Close()

	respBody, err = io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("%s %s: read body: %v", method, target, err)
	}

	return resp.StatusCode, respBody
}

// beginPasskeyCeremony calls a begin endpoint and decodes its options and ceremony token.
func beginPasskeyCeremony(t *testing.T, target, token string) passkeyCeremony {
	t.Helper()

	status, body := postPasskey(t, target, token, "", nil)
	if status != http.StatusOK {
		t.Fatalf("POST %s: expected 200, got %d: %s", target, status, body)
	}

	var ceremony passkeyCeremony

	if err := json.Unmarshal(body, &ceremony); err != nil {
		t.Fatalf("POST %s: decode: %v", target, err)
	}

	if ceremony.CeremonyToken == "" {
		t.Fatalf("POST %s: empty ceremony_token", target)
	}

	return ceremony
}

// addPasskey registers a new virtual credential for the caller and returns the authenticator that
// holds it, with the user handle a discoverable login answers with.
func addPasskey(
	t *testing.T, rp virtualwebauthn.RelyingParty, token string,
) (virtualwebauthn.Authenticator, virtualwebauthn.Credential) {
	t.Helper()

	ceremony := beginPasskeyCeremony(t, basePathV1+"/passkeys/register/begin", token)

	options, err := virtualwebauthn.ParseAttestationOptions(string(ceremony.Options))
	if err != nil {
		t.Fatalf("ParseAttestationOptions: %v", err)
	}

	auth := virtualwebauthn.NewAuthenticatorWithOptions(virtualwebauthn.AuthenticatorOptions{
		UserHandle: []byte(options.UserID),
	})
	cred := virtualwebauthn.NewCredential(virtualwebauthn.KeyTypeEC2)
	attestation := virtualwebauthn.CreateAttestationResponse(rp, auth, cred, *options)

	target := basePathV1 + "/passkeys/register/finish?" + url.Values{"name": {"Virtual key"}}.Encode()

	status, body := postPasskey(t, target, token, ceremony.CeremonyToken, []byte(attestation))
	if status != http.StatusCreated {
		t.Fatalf("finish registration: expected 201, got %d: %s", status, body)
	}

	return auth, cred
}

// listPasskeyNames returns the names of the caller's passkeys.
func listPasskeyNames(t *testing.T, token string) []string {
	t.Helper()

	ctx, cancel := context.WithTimeout(t.Context(), requestTimeout)
	defer cancel()

	resp, err := doAuthenticatedRequest(ctx, http.MethodGet, basePathV1+"/passkeys", http.NoBody, token)
	if err != nil {
		t.Fatalf("list passkeys: %v", err)
	}

	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("list passkeys: expected 200, got %d", resp.StatusCode)
	}

	list := parseJSON[struct {
		Passkeys []struct {
			Name string `json:"name"`
		} `json:"passkeys"`
	}](t, resp)

	names := make([]string, 0, len(list.Passkeys))
	for _, p := range list.Passkeys {
		names = append(names, p.Name)
	}

	return names
}

// loginWithPasskey runs a discoverable login with cred and returns the JWT, plus the ceremony token
// and assertion so the caller can replay them.
func loginWithPasskey(
	t *testing.T, rp virtualwebauthn.RelyingParty, auth *virtualwebauthn.Authenticator, cred virtualwebauthn.Credential,
) (jwt, ceremonyToken string, assertion []byte) {
	t.Helper()

	ceremony := beginPasskeyCeremony(t, basePathV1+"/auth/passkey/begin", "")

	options, err := virtualwebauthn.ParseAssertionOptions(string(ceremony.Options))
	if err != nil {
		t.Fatalf("ParseAssertionOptions: %v", err)
	}

	assertion = []byte(virtualwebauthn.CreateAssertionResponse(rp, *auth, cred, *options))

	status, body := postPasskey(t, passkeyLoginFinish, "", ceremony.CeremonyToken, assertion)
	if status != http.StatusOK {
		t.Fatalf("finish login: expected 200, got %d: %s", status, body)
	}

	var login struct {
		Token string `json:"token"`
	}

	if err := json.Unmarshal(body, &login); err != nil || login.Token == "" {
		t.Fatalf("finish login: decode token: %v (%s)", err, body)
	}

	return login.Token, ceremony.CeremonyToken, assertion
}

// profileUsername returns the username GET /v1/user/profile answers for token.
func profileUsername(t *testing.T, token string) string {
	t.Helper()

	ctx, cancel := context.WithTimeout(t.Context(), requestTimeout)
	defer cancel()

	resp, err := doAuthenticatedRequest(ctx, http.MethodGet, basePathV1+"/user/profile", http.NoBody, token)
	if err != nil {
		t.Fatalf("profile: %v", err)
	}

	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("profile: expected 200, got %d", resp.StatusCode)
	}

	return parseJSON[struct {
		Username string `json:"username"`
	}](t, resp).Username
}

// HTTP: add a passkey, list it, log in with it, use the JWT, and replay the login.
func TestHTTPPasskeyLoginV1(t *testing.T) {
	rp := virtualwebauthn.RelyingParty{ID: rpID, Name: rpName, Origin: rpOrigin}
	passwordJWT := registerAndLogin(t)

	auth, cred := addPasskey(t, rp, passwordJWT)

	if names := listPasskeyNames(t, passwordJWT); len(names) != 1 || names[0] != "Virtual key" {
		t.Fatalf("list passkeys: expected [Virtual key], got %v", names)
	}

	passkeyJWT, ceremonyToken, assertion := loginWithPasskey(t, rp, &auth, cred)

	if got := profileUsername(t, passkeyJWT); got != sanitizeTestName(t) {
		t.Errorf("profile with the passkey JWT: expected username %q, got %q", sanitizeTestName(t), got)
	}

	if status, body := postPasskey(t, passkeyLoginFinish, "", ceremonyToken, assertion); status != http.StatusConflict {
		t.Errorf("replayed finish: expected 409, got %d: %s", status, body)
	}

	// The token is read from the header only: the old query parameter is a missing token.
	inQuery := passkeyLoginFinish + "?" + url.Values{"ceremony_token": {ceremonyToken}}.Encode()
	if status, body := postPasskey(t, inQuery, "", "", assertion); status != http.StatusUnprocessableEntity {
		t.Errorf("finish with the token in the query: expected 422, got %d: %s", status, body)
	}
}

// expectPasskeyStatus sends a request and fails the test unless it answers want.
func expectPasskeyStatus(t *testing.T, what, method, target, token, ceremony string, body []byte, want int) {
	t.Helper()

	if status, respBody := sendPasskey(t, method, target, token, ceremony, body); status != want {
		t.Fatalf("%s: expected %d, got %d: %s", what, want, status, respBody)
	}
}

// registerAndLoginAs creates a user named after the test plus suffix and returns their JWT.
func registerAndLoginAs(t *testing.T, suffix string) string {
	t.Helper()

	name := sanitizeTestName(t) + suffix

	resp := registerUser(t, name, name+"@test.com", testPassword)
	resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("register %s: expected 201, got %d", name, resp.StatusCode)
	}

	return loginUser(t, name+"@test.com", testPassword)
}

// HTTP: rename a passkey, delete it, and check it no longer signs in. Another user's request for
// the passkey is a 404, never a 403, so IDs do not leak.
func TestHTTPPasskeyManageV1(t *testing.T) {
	rp := virtualwebauthn.RelyingParty{ID: rpID, Name: rpName, Origin: rpOrigin}
	ownerJWT := registerAndLogin(t)
	otherJWT := registerAndLoginAs(t, "_other")
	auth, cred := addPasskey(t, rp, ownerJWT)
	target := basePathV1 + "/passkeys/" + base64.RawURLEncoding.EncodeToString(cred.ID)

	expectPasskeyStatus(t, "rename by another user", http.MethodPatch, target, otherJWT,
		"", []byte(`{"name":"Stolen"}`), http.StatusNotFound)
	expectPasskeyStatus(t, "rename", http.MethodPatch, target, ownerJWT,
		"", []byte(`{"name":"  Work laptop "}`), http.StatusOK)

	if names := listPasskeyNames(t, ownerJWT); len(names) != 1 || names[0] != "Work laptop" {
		t.Fatalf("list after rename: expected [Work laptop], got %v", names)
	}

	expectPasskeyStatus(t, "delete by another user", http.MethodDelete, target, otherJWT, "", nil, http.StatusNotFound)
	expectPasskeyStatus(t, "delete", http.MethodDelete, target, ownerJWT, "", nil, http.StatusNoContent)
	expectPasskeyStatus(t, "delete again", http.MethodDelete, target, ownerJWT, "", nil, http.StatusNotFound)

	if names := listPasskeyNames(t, ownerJWT); len(names) != 0 {
		t.Errorf("list after delete: expected none, got %v", names)
	}

	ceremony := beginPasskeyCeremony(t, basePathV1+"/auth/passkey/begin", "")

	options, err := virtualwebauthn.ParseAssertionOptions(string(ceremony.Options))
	if err != nil {
		t.Fatalf("ParseAssertionOptions: %v", err)
	}

	assertion := virtualwebauthn.CreateAssertionResponse(rp, auth, cred, *options)

	expectPasskeyStatus(t, "login with the deleted passkey", http.MethodPost, passkeyLoginFinish, "",
		ceremony.CeremonyToken, []byte(assertion), http.StatusUnauthorized)
}
