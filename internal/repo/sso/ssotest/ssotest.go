// Package ssotest is an OpenID Connect provider on httptest for the SSO tests: discovery, JWKS, and
// a token endpoint that checks the client secret, the code and its PKCE verifier, and signs a real
// RS256 ID token. A test can make any endpoint fail as an outage would.
package ssotest

import (
	"cmp"
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

// Endpoint is one of the provider's endpoints, which Fail makes fail.
type Endpoint int

const (
	// Discovery serves the provider's metadata.
	Discovery Endpoint = iota
	// Token exchanges a code for tokens.
	Token
	// Keys serves the keys the provider signs ID tokens with.
	Keys
)

// Fault is how Fail makes an endpoint fail.
type Fault int

const (
	// Answers is an endpoint that works, as each one does until Fail says otherwise.
	Answers Fault = iota
	// HangsUp closes the connection without an answer, as a provider that cannot be reached.
	HangsUp
	// Fails answers 500, as a provider that is up but failing.
	Fails
	// Refuses answers 400 invalid_grant, the OAuth error for a code the provider turns down.
	Refuses
	// Stalls answers nothing until the caller gives up, as a provider that does not answer in time.
	// Stalled receives each request it holds.
	Stalls
)

// IDP is the fake provider.
type IDP struct {
	t   *testing.T
	srv *httptest.Server
	// key signs the ID tokens and is the key the provider publishes.
	key *rsa.PrivateKey
	// stalled receives a request a Stalls fault holds, as it starts holding it.
	stalled chan struct{}
	// released ends the requests a Stalls fault holds, when the test ends.
	released chan struct{}

	mu     sync.Mutex
	codes  map[string]grant
	faults map[Endpoint]Fault
	// forged, once Forge sets it, signs the ID tokens in place of key.
	forged *rsa.PrivateKey
}

// grant is what the fake remembers about a code it handed out.
type grant struct {
	challenge string
	claims    map[string]any
}

// New starts a provider for the length of the test.
func New(t *testing.T) *IDP {
	t.Helper()

	idp := &IDP{
		t: t, key: newKey(t), stalled: make(chan struct{}, 1), released: make(chan struct{}),
		codes: map[string]grant{}, faults: map[Endpoint]Fault{},
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /.well-known/openid-configuration", idp.faulty(Discovery, idp.discovery))
	mux.HandleFunc("GET /jwks", idp.faulty(Keys, idp.jwks))
	mux.HandleFunc("POST /token", idp.faulty(Token, idp.token))

	idp.srv = httptest.NewServer(mux)
	t.Cleanup(idp.srv.Close)
	// Cleanups run last first: the stalled requests end before Close waits for them.
	t.Cleanup(func() { close(idp.released) })

	return idp
}

func newKey(t *testing.T) *rsa.PrivateKey {
	t.Helper()

	key, err := rsa.GenerateKey(rand.Reader, _keyBits)
	require.NoError(t, err)

	return key
}

func (p *IDP) Issuer() string { return p.srv.URL }

// Fail makes endpoint e fail as f says from now on; Answers makes it work again.
func (p *IDP) Fail(e Endpoint, f Fault) {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.faults[e] = f
}

// Stalled receives a request a Stalls fault holds, as it starts holding it.
func (p *IDP) Stalled() <-chan struct{} { return p.stalled }

// Forge makes the provider sign every ID token from now on with a key it does not publish, under the
// published key's ID.
func (p *IDP) Forge() {
	key := newKey(p.t)

	p.mu.Lock()
	defer p.mu.Unlock()

	p.forged = key
}

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

// faulty serves h, unless Fail made endpoint e fail.
func (p *IDP) faulty(e Endpoint, h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		p.mu.Lock()
		fault := p.faults[e]
		p.mu.Unlock()

		switch fault {
		case Answers:
			h(w, r)
		case HangsUp:
			conn, _, err := http.NewResponseController(w).Hijack()
			require.NoError(p.t, err)
			require.NoError(p.t, conn.Close())
		case Fails:
			w.WriteHeader(http.StatusInternalServerError)
		case Refuses:
			p.writeError(w, "invalid_grant")
		case Stalls:
			select {
			case p.stalled <- struct{}{}:
			default:
			}

			select {
			case <-r.Context().Done():
			case <-p.released:
			}
		}
	}
}

func (p *IDP) discovery(w http.ResponseWriter, _ *http.Request) {
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

	p.mu.Lock()
	key := cmp.Or(p.forged, p.key)
	p.mu.Unlock()

	signed, err := token.SignedString(key)
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
