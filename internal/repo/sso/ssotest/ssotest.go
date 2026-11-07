// Package ssotest is an OpenID Connect provider on httptest for the SSO tests: discovery, JWKS, and
// a token endpoint that checks the client secret, the code and its PKCE verifier, and signs a real
// RS256 ID token.
package ssotest

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"math/big"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	jwtlib "github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/require"
)

// The client the tests sign in as, and the subject the provider signs in by default.
const (
	ClientID     = "go-clean-template"
	ClientSecret = "client-secret"
	BaseURL      = "http://localhost"
	RedirectURL  = BaseURL + "/sso/callback"
	Subject      = "idp-subject-1"
	Email        = "alice@example.com"
	Username     = "alice"

	_keyID   = "test-key"
	_keyBits = 2048
	_codeTTL = 5 * time.Minute
)

// IDP is the fake provider. Down makes discovery fail, as a provider that is still starting does.
type IDP struct {
	Down atomic.Bool

	t   *testing.T
	srv *httptest.Server
	key *rsa.PrivateKey

	mu    sync.Mutex
	codes map[string]grant
}

// grant is what the fake remembers about a code it handed out.
type grant struct {
	challenge string
	claims    map[string]any
}

// New starts a provider for the length of the test.
func New(t *testing.T) *IDP {
	t.Helper()

	key, err := rsa.GenerateKey(rand.Reader, _keyBits)
	require.NoError(t, err)

	idp := &IDP{t: t, key: key, codes: map[string]grant{}}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /.well-known/openid-configuration", idp.discovery)
	mux.HandleFunc("GET /jwks", idp.jwks)
	mux.HandleFunc("POST /token", idp.token)

	idp.srv = httptest.NewServer(mux)
	t.Cleanup(idp.srv.Close)

	return idp
}

func (p *IDP) Issuer() string { return p.srv.URL }

// Authorize plays the user signing in at the provider: it reads the nonce and PKCE challenge from
// the authorize URL and returns a code whose ID token carries claims on top of valid defaults. A
// claim set to nil is left out.
func (p *IDP) Authorize(authURL string, claims map[string]any) (code, state string) {
	u, err := url.Parse(authURL)
	require.NoError(p.t, err)

	q := u.Query()
	now := time.Now()
	all := map[string]any{
		"iss": p.Issuer(), "aud": ClientID, "sub": Subject,
		"iat": now.Unix(), "exp": now.Add(_codeTTL).Unix(),
		"nonce": q.Get("nonce"), "email": Email, "preferred_username": Username,
	}

	for k, v := range claims {
		if v == nil {
			delete(all, k)
		} else {
			all[k] = v
		}
	}

	code = "code-" + rand.Text()

	p.mu.Lock()
	p.codes[code] = grant{challenge: q.Get("code_challenge"), claims: all}
	p.mu.Unlock()

	return code, q.Get("state")
}

func (p *IDP) discovery(w http.ResponseWriter, _ *http.Request) {
	if p.Down.Load() {
		w.WriteHeader(http.StatusServiceUnavailable)

		return
	}

	p.writeJSON(w, map[string]any{
		"issuer":                                p.Issuer(),
		"authorization_endpoint":                p.Issuer() + "/auth",
		"token_endpoint":                        p.Issuer() + "/token",
		"jwks_uri":                              p.Issuer() + "/jwks",
		"id_token_signing_alg_values_supported": []string{"RS256"},
	})
}

func (p *IDP) jwks(w http.ResponseWriter, _ *http.Request) {
	pub := p.key.PublicKey
	p.writeJSON(w, map[string]any{"keys": []map[string]any{{
		"kty": "RSA", "use": "sig", "alg": "RS256", "kid": _keyID,
		"n": base64.RawURLEncoding.EncodeToString(pub.N.Bytes()),
		"e": base64.RawURLEncoding.EncodeToString(big.NewInt(int64(pub.E)).Bytes()),
	}}})
}

func (p *IDP) token(w http.ResponseWriter, r *http.Request) {
	id, secret, ok := r.BasicAuth()
	if !ok {
		id, secret = r.FormValue("client_id"), r.FormValue("client_secret")
	}

	p.mu.Lock()
	g, known := p.codes[r.FormValue("code")]
	delete(p.codes, r.FormValue("code"))
	p.mu.Unlock()

	sum := sha256.Sum256([]byte(r.FormValue("code_verifier")))

	switch {
	case id != ClientID || secret != ClientSecret:
		p.writeError(w, "invalid_client")
	case !known, r.FormValue("redirect_uri") != RedirectURL:
		p.writeError(w, "invalid_grant")
	case base64.RawURLEncoding.EncodeToString(sum[:]) != g.challenge:
		p.writeError(w, "invalid_grant")
	default:
		p.writeJSON(w, map[string]any{
			"access_token": "access", "token_type": "Bearer", "expires_in": int(_codeTTL.Seconds()),
			"id_token": p.sign(g.claims),
		})
	}
}

func (p *IDP) sign(claims map[string]any) string {
	token := jwtlib.NewWithClaims(jwtlib.SigningMethodRS256, jwtlib.MapClaims(claims))
	token.Header["kid"] = _keyID

	signed, err := token.SignedString(p.key)
	require.NoError(p.t, err)

	return signed
}

func (p *IDP) writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	require.NoError(p.t, json.NewEncoder(w).Encode(v))
}

func (p *IDP) writeError(w http.ResponseWriter, code string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusBadRequest)
	require.NoError(p.t, json.NewEncoder(w).Encode(map[string]string{"error": code}))
}
