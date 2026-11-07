package sso_test

import (
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/m7medvision/go-clean-template/config"
	"github.com/m7medvision/go-clean-template/internal/entity"
	"github.com/m7medvision/go-clean-template/internal/repo/sso"
	"github.com/m7medvision/go-clean-template/internal/repo/sso/ssotest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newSSO(t *testing.T, idp *ssotest.IDP, configure ...func(*config.OIDC)) *sso.SSO {
	t.Helper()

	cfg := config.OIDC{
		Issuer: idp.Issuer(), ClientID: ssotest.ClientID, ClientSecret: ssotest.ClientSecret,
		RolesClaim: "realm_access.roles",
		RoleMap:    map[string]string{"app-admin": "admin", "app-user": "user"},
	}
	for _, c := range configure {
		c(&cfg)
	}

	s, err := sso.New("test-secret", cfg, ssotest.RedirectURL)
	require.NoError(t, err)

	return s
}

// signIn runs Begin, the provider sign-in with claims, and Verify.
func signIn(t *testing.T, s *sso.SSO, idp *ssotest.IDP, claims map[string]any) (entity.SSOIdentity, error) {
	t.Helper()

	authURL, token, err := s.Begin(t.Context())
	require.NoError(t, err)

	code, state := idp.Authorize(authURL, claims)

	return s.Verify(t.Context(), code, state, token)
}

func TestDisabledSSO(t *testing.T) {
	t.Parallel()

	s := newSSO(t, ssotest.New(t), func(c *config.OIDC) { c.Issuer = "" })

	assert.False(t, s.Enabled())

	_, _, err := s.Begin(t.Context())
	require.ErrorIs(t, err, entity.ErrOIDCDisabled)

	_, err = s.Verify(t.Context(), "code", "state", "token")
	require.ErrorIs(t, err, entity.ErrOIDCDisabled)
}

func TestBeginSendsTheBrowserToTheProvider(t *testing.T) {
	t.Parallel()

	idp := ssotest.New(t)
	s := newSSO(t, idp)
	assert.True(t, s.Enabled())

	authURL, token, err := s.Begin(t.Context())
	require.NoError(t, err)
	assert.NotEmpty(t, token)

	u, err := url.Parse(authURL)
	require.NoError(t, err)
	assert.Equal(t, idp.Issuer()+"/auth", u.Scheme+"://"+u.Host+u.Path)

	q := u.Query()
	assert.Equal(t, ssotest.ClientID, q.Get("client_id"))
	assert.Equal(t, ssotest.RedirectURL, q.Get("redirect_uri"))
	assert.Equal(t, "code", q.Get("response_type"))
	assert.Contains(t, strings.Fields(q.Get("scope")), "openid")
	assert.Contains(t, strings.Fields(q.Get("scope")), "email")
	assert.Equal(t, "S256", q.Get("code_challenge_method"))
	assert.NotEmpty(t, q.Get("code_challenge"))
	assert.NotEmpty(t, q.Get("state"))
	assert.NotEmpty(t, q.Get("nonce"))

	again, _, err := s.Begin(t.Context())
	require.NoError(t, err)

	u2, err := url.Parse(again)
	require.NoError(t, err)
	assert.NotEqual(t, q.Get("state"), u2.Query().Get("state"), "every sign-in gets a fresh state")
	assert.NotEqual(t, q.Get("nonce"), u2.Query().Get("nonce"), "every sign-in gets a fresh nonce")
}

func TestBeginRetriesDiscovery(t *testing.T) {
	t.Parallel()

	idp := ssotest.New(t)
	s := newSSO(t, idp)

	// The provider may still be starting when the first sign-in arrives.
	idp.Down.Store(true)

	_, _, err := s.Begin(t.Context())
	require.ErrorIs(t, err, entity.ErrOIDCUnavailable)

	idp.Down.Store(false)

	_, _, err = s.Begin(t.Context())
	require.NoError(t, err, "a failed discovery is not cached")
}

func TestVerifyReturnsTheIdentityThatSignedIn(t *testing.T) {
	t.Parallel()

	idp := ssotest.New(t)

	identity, err := signIn(t, newSSO(t, idp), idp, nil)
	require.NoError(t, err)

	assert.Equal(t, entity.SSOIdentity{
		Issuer: idp.Issuer(), Subject: ssotest.Subject, Email: ssotest.Email, Username: ssotest.Username,
		Roles: []entity.Role{entity.RoleUser},
	}, identity)
}

func TestAnIdentityWithoutAUsernameIsNamedByItsEmail(t *testing.T) {
	t.Parallel()

	idp := ssotest.New(t)

	identity, err := signIn(t, newSSO(t, idp), idp, map[string]any{"preferred_username": nil})
	require.NoError(t, err)

	assert.Equal(t, ssotest.Email, identity.Username)
}

func TestVerifyMapsTheProvidersRoles(t *testing.T) {
	t.Parallel()

	keycloak := func(*config.OIDC) {}
	entra := func(c *config.OIDC) {
		c.RolesClaim = "roles"
		c.RoleMap = map[string]string{"Tasks.Admin": "admin"}
	}
	okta := func(c *config.OIDC) {
		c.RolesClaim = "groups"
		c.RoleMap = map[string]string{"task-admins": "admin", "everyone": "user"}
	}

	tests := []struct {
		name      string
		configure func(*config.OIDC)
		claims    map[string]any
		want      []entity.Role
	}{
		{
			"Keycloak realm roles", keycloak,
			map[string]any{"realm_access": map[string]any{"roles": []string{"app-admin", "app-user", "offline_access"}}},
			[]entity.Role{entity.RoleAdmin, entity.RoleUser},
		},
		{"Entra ID app roles", entra, map[string]any{"roles": []string{"Tasks.Admin"}}, []entity.Role{entity.RoleAdmin}},
		{
			"Okta groups", okta,
			map[string]any{"groups": []string{"everyone", "task-admins"}},
			[]entity.Role{entity.RoleUser, entity.RoleAdmin},
		},
		{"a single string claim", entra, map[string]any{"roles": "Tasks.Admin"}, []entity.Role{entity.RoleAdmin}},
		{
			"no mapped role falls back to user", keycloak,
			map[string]any{"realm_access": map[string]any{"roles": []string{"offline_access"}}},
			[]entity.Role{entity.RoleUser},
		},
		{"a missing claim falls back to user", keycloak, nil, []entity.Role{entity.RoleUser}},
		{"a claim of the wrong type falls back to user", keycloak, map[string]any{"realm_access": 42}, []entity.Role{entity.RoleUser}},
		{
			"two names for one role grant it once", okta,
			map[string]any{"groups": []string{"everyone", "task-admins", "everyone"}},
			[]entity.Role{entity.RoleUser, entity.RoleAdmin},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			idp := ssotest.New(t)

			identity, err := signIn(t, newSSO(t, idp, tt.configure), idp, tt.claims)
			require.NoError(t, err)
			assert.Equal(t, tt.want, identity.Roles)
		})
	}
}

// A role map naming a role the app does not have would grant nothing on every sign-in, so it stops
// the app at startup.
func TestNewRefusesARoleTheAppDoesNotHave(t *testing.T) {
	t.Parallel()

	for _, value := range []string{"superuser", "Admin", ""} {
		_, err := sso.New("test-secret", config.OIDC{
			Issuer:  "https://idp.example",
			RoleMap: map[string]string{"app-user": "user", "root": value},
		}, ssotest.RedirectURL)
		require.ErrorIs(t, err, sso.ErrUnknownRole, "role %q", value)
	}
}

func TestVerifyRefusesAnInvalidCeremony(t *testing.T) {
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

			idp := ssotest.New(t)
			s := newSSO(t, idp)

			authURL, token, err := s.Begin(t.Context())
			require.NoError(t, err)

			code, state := idp.Authorize(authURL, tt.claims)
			if tt.tamper != nil {
				code, state, token = tt.tamper(code, state, token)
			}

			_, err = s.Verify(t.Context(), code, state, token)
			require.ErrorIs(t, err, entity.ErrOIDCCeremonyInvalid)
		})
	}
}
