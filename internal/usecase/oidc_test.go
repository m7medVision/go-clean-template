package usecase_test

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"math/big"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	jwtlib "github.com/golang-jwt/jwt/v5"
	"github.com/m7medvision/go-clean-template/internal/entity"
	"github.com/m7medvision/go-clean-template/internal/usecase"
	"github.com/m7medvision/go-clean-template/internal/usecase/oidc"
	"github.com/m7medvision/go-clean-template/pkg/jwt"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

//nolint:gosec // test fixtures, not credentials
const (
	_oidcClientID     = "go-clean-template"
	_oidcClientSecret = "client-secret"
	_oidcRedirectURL  = "http://localhost/sso/callback"
	_oidcKeyID        = "test-key"
	_oidcSecret       = "test-secret"
)

// fakeIDP is an OpenID Connect provider on httptest: discovery, JWKS, and a token endpoint that
// checks the client secret, the code and its PKCE verifier, and signs a real RS256 ID token.
type fakeIDP struct {
	t    *testing.T
	srv  *httptest.Server
	key  *rsa.PrivateKey
	down atomic.Bool

	mu    sync.Mutex
	codes map[string]grant
}

// grant is what the fake remembers about a code it handed out.
type grant struct {
	challenge string
	claims    map[string]any
}

func newFakeIDP(t *testing.T) *fakeIDP {
	t.Helper()

	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)

	idp := &fakeIDP{t: t, key: key, codes: map[string]grant{}}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /.well-known/openid-configuration", idp.discovery)
	mux.HandleFunc("GET /jwks", idp.jwks)
	mux.HandleFunc("POST /token", idp.token)

	idp.srv = httptest.NewServer(mux)
	t.Cleanup(idp.srv.Close)

	return idp
}

func (p *fakeIDP) issuer() string { return p.srv.URL }

func (p *fakeIDP) discovery(w http.ResponseWriter, _ *http.Request) {
	if p.down.Load() {
		w.WriteHeader(http.StatusServiceUnavailable)

		return
	}

	p.writeJSON(w, map[string]any{
		"issuer":                                p.issuer(),
		"authorization_endpoint":                p.issuer() + "/auth",
		"token_endpoint":                        p.issuer() + "/token",
		"jwks_uri":                              p.issuer() + "/jwks",
		"id_token_signing_alg_values_supported": []string{"RS256"},
	})
}

func (p *fakeIDP) jwks(w http.ResponseWriter, _ *http.Request) {
	pub := p.key.PublicKey
	p.writeJSON(w, map[string]any{"keys": []map[string]any{{
		"kty": "RSA", "use": "sig", "alg": "RS256", "kid": _oidcKeyID,
		"n": base64.RawURLEncoding.EncodeToString(pub.N.Bytes()),
		"e": base64.RawURLEncoding.EncodeToString(big.NewInt(int64(pub.E)).Bytes()),
	}}})
}

func (p *fakeIDP) token(w http.ResponseWriter, r *http.Request) {
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
	case id != _oidcClientID || secret != _oidcClientSecret:
		p.writeError(w, "invalid_client")
	case !known, r.FormValue("redirect_uri") != _oidcRedirectURL:
		p.writeError(w, "invalid_grant")
	case base64.RawURLEncoding.EncodeToString(sum[:]) != g.challenge:
		p.writeError(w, "invalid_grant")
	default:
		p.writeJSON(w, map[string]any{
			"access_token": "access", "token_type": "Bearer", "expires_in": 300,
			"id_token": p.sign(g.claims),
		})
	}
}

func (p *fakeIDP) sign(claims map[string]any) string {
	token := jwtlib.NewWithClaims(jwtlib.SigningMethodRS256, jwtlib.MapClaims(claims))
	token.Header["kid"] = _oidcKeyID

	signed, err := token.SignedString(p.key)
	require.NoError(p.t, err)

	return signed
}

func (p *fakeIDP) writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	require.NoError(p.t, json.NewEncoder(w).Encode(v))
}

func (p *fakeIDP) writeError(w http.ResponseWriter, code string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusBadRequest)
	require.NoError(p.t, json.NewEncoder(w).Encode(map[string]string{"error": code}))
}

// authorize plays the user signing in at the provider: it reads the nonce and PKCE challenge from
// the authorize URL and returns a code whose ID token carries claims on top of valid defaults. A
// claim set to nil is left out.
func (p *fakeIDP) authorize(authURL string, claims map[string]any) (code, state string) {
	u, err := url.Parse(authURL)
	require.NoError(p.t, err)

	q := u.Query()
	now := time.Now()
	all := map[string]any{
		"iss": p.issuer(), "aud": _oidcClientID, "sub": "idp-subject-1",
		"iat": now.Unix(), "exp": now.Add(5 * time.Minute).Unix(),
		"nonce": q.Get("nonce"), "email": "alice@example.com", "preferred_username": "alice",
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

type oidcFixture struct {
	uc         usecase.OIDC
	idp        *fakeIDP
	identities *MockIdentityRepo
	tokens     *jwt.Manager
}

func newOIDCFixture(t *testing.T, configure ...func(*oidc.Config)) oidcFixture {
	t.Helper()

	idp := newFakeIDP(t)
	identities := NewMockIdentityRepo(gomock.NewController(t))
	tokens := jwt.New(_oidcSecret, time.Hour)

	state, err := jwt.NewScoped(_oidcSecret, "oidc-state", 10*time.Minute)
	require.NoError(t, err)

	cfg := &oidc.Config{
		Issuer: idp.issuer(), ClientID: _oidcClientID, ClientSecret: _oidcClientSecret,
		RedirectURL: _oidcRedirectURL, RolesClaim: "realm_access.roles",
		RoleMap: map[string]string{"app-admin": "admin", "app-user": "user"},
	}
	for _, c := range configure {
		c(cfg)
	}

	return oidcFixture{uc: oidc.New(identities, state, tokens, cfg), idp: idp, identities: identities, tokens: tokens}
}

// signIn runs Begin, the provider sign-in with claims, and Finish.
func (f oidcFixture) signIn(t *testing.T, claims map[string]any) (string, error) {
	t.Helper()

	authURL, token, err := f.uc.Begin(t.Context())
	require.NoError(t, err)

	code, state := f.idp.authorize(authURL, claims)

	return f.uc.Finish(t.Context(), code, state, token)
}

func (f oidcFixture) roles(t *testing.T, authToken string) []string {
	t.Helper()

	claims, err := f.tokens.Parse(authToken)
	require.NoError(t, err)

	return claims.Roles
}

func knownUser(f oidcFixture) {
	f.identities.EXPECT().UserByIdentity(gomock.Any(), f.idp.issuer(), "idp-subject-1").
		Return(entity.User{ID: "user-1", Username: "alice", Email: "alice@example.com"}, nil)
}

func TestOIDCDisabled(t *testing.T) {
	t.Parallel()

	f := newOIDCFixture(t, func(c *oidc.Config) { c.Issuer = "" })

	assert.False(t, f.uc.Enabled())

	_, _, err := f.uc.Begin(t.Context())
	require.ErrorIs(t, err, entity.ErrOIDCDisabled)

	_, err = f.uc.Finish(t.Context(), "code", "state", "token")
	require.ErrorIs(t, err, entity.ErrOIDCDisabled)
}

func TestOIDCBegin(t *testing.T) {
	t.Parallel()

	f := newOIDCFixture(t)
	assert.True(t, f.uc.Enabled())

	authURL, token, err := f.uc.Begin(t.Context())
	require.NoError(t, err)
	assert.NotEmpty(t, token)

	u, err := url.Parse(authURL)
	require.NoError(t, err)
	assert.Equal(t, f.idp.issuer()+"/auth", u.Scheme+"://"+u.Host+u.Path)

	q := u.Query()
	assert.Equal(t, _oidcClientID, q.Get("client_id"))
	assert.Equal(t, _oidcRedirectURL, q.Get("redirect_uri"))
	assert.Equal(t, "code", q.Get("response_type"))
	assert.Contains(t, strings.Fields(q.Get("scope")), "openid")
	assert.Contains(t, strings.Fields(q.Get("scope")), "email")
	assert.Equal(t, "S256", q.Get("code_challenge_method"))
	assert.NotEmpty(t, q.Get("code_challenge"))
	assert.NotEmpty(t, q.Get("state"))
	assert.NotEmpty(t, q.Get("nonce"))

	again, _, err := f.uc.Begin(t.Context())
	require.NoError(t, err)

	u2, err := url.Parse(again)
	require.NoError(t, err)
	assert.NotEqual(t, q.Get("state"), u2.Query().Get("state"), "every sign-in gets a fresh state")
	assert.NotEqual(t, q.Get("nonce"), u2.Query().Get("nonce"), "every sign-in gets a fresh nonce")
}

func TestOIDCBegin_RetriesDiscovery(t *testing.T) {
	t.Parallel()

	f := newOIDCFixture(t)

	// The provider may still be starting when the first sign-in arrives.
	f.idp.down.Store(true)

	_, _, err := f.uc.Begin(t.Context())
	require.ErrorIs(t, err, entity.ErrOIDCUnavailable)

	f.idp.down.Store(false)

	_, _, err = f.uc.Begin(t.Context())
	require.NoError(t, err, "a failed discovery is not cached")
}

func TestOIDCFinish_FirstSignInCreatesUser(t *testing.T) {
	t.Parallel()

	f := newOIDCFixture(t)

	var created entity.User

	gomock.InOrder(
		f.identities.EXPECT().UserByIdentity(gomock.Any(), f.idp.issuer(), "idp-subject-1").
			Return(entity.User{}, entity.ErrUserNotFound),
		f.identities.EXPECT().CreateWithIdentity(gomock.Any(), gomock.Any(), f.idp.issuer(), "idp-subject-1").
			DoAndReturn(func(_ context.Context, u *entity.User, _, _ string) error {
				created = *u

				return nil
			}),
	)

	authToken, err := f.signIn(t, map[string]any{"realm_access": map[string]any{"roles": []string{"app-admin"}}})
	require.NoError(t, err)

	assert.NotEmpty(t, created.ID)
	assert.Equal(t, "alice", created.Username)
	assert.Equal(t, "alice@example.com", created.Email)
	assert.Empty(t, created.PasswordHash, "an SSO user has no password")

	claims, err := f.tokens.Parse(authToken)
	require.NoError(t, err)
	assert.Equal(t, created.ID, claims.UserID)
	assert.Equal(t, []string{"admin"}, claims.Roles)
}

func TestOIDCFinish_KnownIdentityReusesUser(t *testing.T) {
	t.Parallel()

	f := newOIDCFixture(t)
	knownUser(f)

	authToken, err := f.signIn(t, nil)
	require.NoError(t, err)

	claims, err := f.tokens.Parse(authToken)
	require.NoError(t, err)
	assert.Equal(t, "user-1", claims.UserID)
}

func TestOIDCFinish_LinkRaceReadsTheWinner(t *testing.T) {
	t.Parallel()

	f := newOIDCFixture(t)

	gomock.InOrder(
		f.identities.EXPECT().UserByIdentity(gomock.Any(), gomock.Any(), gomock.Any()).
			Return(entity.User{}, entity.ErrUserNotFound),
		f.identities.EXPECT().CreateWithIdentity(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
			Return(entity.ErrUserAlreadyExists),
		f.identities.EXPECT().UserByIdentity(gomock.Any(), gomock.Any(), gomock.Any()).
			Return(entity.User{ID: "user-winner"}, nil),
	)

	authToken, err := f.signIn(t, nil)
	require.NoError(t, err)

	claims, err := f.tokens.Parse(authToken)
	require.NoError(t, err)
	assert.Equal(t, "user-winner", claims.UserID)
}

func TestOIDCFinish_AccountConflict(t *testing.T) {
	t.Parallel()

	f := newOIDCFixture(t)

	f.identities.EXPECT().UserByIdentity(gomock.Any(), gomock.Any(), gomock.Any()).
		Return(entity.User{}, entity.ErrUserNotFound)
	f.identities.EXPECT().CreateWithIdentity(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		Return(entity.ErrAccountConflict)

	_, err := f.signIn(t, nil)
	require.ErrorIs(t, err, entity.ErrAccountConflict)
}

func TestOIDCFinish_RepoError(t *testing.T) {
	t.Parallel()

	f := newOIDCFixture(t)

	f.identities.EXPECT().UserByIdentity(gomock.Any(), gomock.Any(), gomock.Any()).
		Return(entity.User{}, errInternalServErr)

	_, err := f.signIn(t, nil)
	require.ErrorIs(t, err, errInternalServErr)
}

func TestOIDCFinish_Roles(t *testing.T) {
	t.Parallel()

	keycloak := func(*oidc.Config) {}
	entra := func(c *oidc.Config) {
		c.RolesClaim = "roles"
		c.RoleMap = map[string]string{"Tasks.Admin": "admin"}
	}
	okta := func(c *oidc.Config) {
		c.RolesClaim = "groups"
		c.RoleMap = map[string]string{"task-admins": "admin", "everyone": "user"}
	}

	tests := []struct {
		name      string
		configure func(*oidc.Config)
		claims    map[string]any
		want      []string
	}{
		{
			"Keycloak realm roles", keycloak,
			map[string]any{"realm_access": map[string]any{"roles": []string{"app-admin", "app-user", "offline_access"}}},
			[]string{"admin", "user"},
		},
		{"Entra ID app roles", entra, map[string]any{"roles": []string{"Tasks.Admin"}}, []string{"admin"}},
		{"Okta groups", okta, map[string]any{"groups": []string{"everyone", "task-admins"}}, []string{"user", "admin"}},
		{"a single string claim", entra, map[string]any{"roles": "Tasks.Admin"}, []string{"admin"}},
		{
			"no mapped role falls back to user", keycloak,
			map[string]any{"realm_access": map[string]any{"roles": []string{"offline_access"}}},
			[]string{"user"},
		},
		{"a missing claim falls back to user", keycloak, nil, []string{"user"}},
		{"a claim of the wrong type falls back to user", keycloak, map[string]any{"realm_access": 42}, []string{"user"}},
		{"a map value that is no app role is dropped", func(c *oidc.Config) {
			c.RolesClaim = "roles"
			c.RoleMap = map[string]string{"root": "superuser"}
		}, map[string]any{"roles": []string{"root"}}, []string{"user"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			f := newOIDCFixture(t, tt.configure)
			knownUser(f)

			authToken, err := f.signIn(t, tt.claims)
			require.NoError(t, err)
			assert.Equal(t, tt.want, f.roles(t, authToken))
		})
	}
}

func TestOIDCFinish_CeremonyInvalid(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		claims map[string]any
		tamper func(code, state, token string) (string, string, string)
	}{
		{"a state that does not match", nil, func(c, _, tk string) (string, string, string) { return c, "forged", tk }},
		{"a forged ceremony token", nil, func(c, s, _ string) (string, string, string) { return c, s, "not-a-token" }},
		{"a code the provider never issued", nil, func(_, s, tk string) (string, string, string) { return "unknown", s, tk }},
		{"an ID token for another nonce", map[string]any{"nonce": "replayed"}, nil},
		{"an ID token for another client", map[string]any{"aud": "someone-else"}, nil},
		{"an ID token from another issuer", map[string]any{"iss": "https://evil.example"}, nil},
		{"an expired ID token", map[string]any{"exp": time.Now().Add(-time.Hour).Unix()}, nil},
		{"an ID token without an email", map[string]any{"email": nil}, nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// The identities mock expects nothing: a failed ceremony never touches the users.
			f := newOIDCFixture(t)

			authURL, token, err := f.uc.Begin(t.Context())
			require.NoError(t, err)

			code, state := f.idp.authorize(authURL, tt.claims)
			if tt.tamper != nil {
				code, state, token = tt.tamper(code, state, token)
			}

			_, err = f.uc.Finish(t.Context(), code, state, token)
			require.ErrorIs(t, err, entity.ErrOIDCCeremonyInvalid)
		})
	}
}

func TestOIDCFinish_CodeWorksOnce(t *testing.T) {
	t.Parallel()

	f := newOIDCFixture(t)
	knownUser(f)

	authURL, token, err := f.uc.Begin(t.Context())
	require.NoError(t, err)

	code, state := f.idp.authorize(authURL, nil)

	_, err = f.uc.Finish(t.Context(), code, state, token)
	require.NoError(t, err)

	_, err = f.uc.Finish(t.Context(), code, state, token)
	require.ErrorIs(t, err, entity.ErrOIDCCeremonyInvalid)
}
