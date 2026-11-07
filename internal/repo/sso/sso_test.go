package sso_test

import (
	"context"
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

// _stall bounds a step's calls to a provider that stalls, so the test does not wait out sso.Timeout.
const _stall = 300 * time.Millisecond

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
	idp.Fail(ssotest.Discovery, ssotest.Fails)

	_, _, err := s.Begin(t.Context())
	require.ErrorIs(t, err, entity.ErrOIDCUnavailable)

	idp.Fail(ssotest.Discovery, ssotest.Answers)

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

// Keys the provider serves that verify none of its ID token are the token's fault, not an outage.
func TestVerifyRefusesAnIDTokenTheProviderDidNotSign(t *testing.T) {
	t.Parallel()

	idp := ssotest.New(t)
	idp.Forge()

	_, err := signIn(t, newSSO(t, idp), idp, nil)
	require.ErrorIs(t, err, entity.ErrOIDCCeremonyInvalid)
}

// finishElsewhere begins a sign-in at idp and has the provider sign the user in, then breaks endpoint
// with fault and returns a second SSO to finish on, as after a restart: it discovers idp again.
func finishElsewhere(
	t *testing.T, idp *ssotest.IDP, endpoint ssotest.Endpoint, fault ssotest.Fault,
) (s *sso.SSO, code, state, token string) {
	t.Helper()

	authURL, token, err := newSSO(t, idp).Begin(t.Context())
	require.NoError(t, err)

	code, state = idp.Authorize(authURL, nil)
	idp.Fail(endpoint, fault)

	return newSSO(t, idp), code, state, token
}

// Finishing a sign-in calls the provider three times: discovery, the code exchange, and the key fetch
// that verifies the ID token. At each, the provider failing is an outage (502), and only the provider
// turning the code down is the sign-in's fault (401).
func TestVerifyTellsAnOutageFromARefusal(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		endpoint ssotest.Endpoint
		fault    ssotest.Fault
		want     error
	}{
		{"discovery cannot be reached", ssotest.Discovery, ssotest.HangsUp, entity.ErrOIDCUnavailable},
		{"discovery fails", ssotest.Discovery, ssotest.Fails, entity.ErrOIDCUnavailable},
		{"discovery answers 400", ssotest.Discovery, ssotest.Refuses, entity.ErrOIDCUnavailable},
		{"discovery does not answer in time", ssotest.Discovery, ssotest.Stalls, entity.ErrOIDCUnavailable},
		{"the token endpoint cannot be reached", ssotest.Token, ssotest.HangsUp, entity.ErrOIDCUnavailable},
		{"the token endpoint fails", ssotest.Token, ssotest.Fails, entity.ErrOIDCUnavailable},
		{"the token endpoint refuses the code", ssotest.Token, ssotest.Refuses, entity.ErrOIDCCeremonyInvalid},
		{"the token endpoint does not answer in time", ssotest.Token, ssotest.Stalls, entity.ErrOIDCUnavailable},
		{"the keys cannot be reached", ssotest.Keys, ssotest.HangsUp, entity.ErrOIDCUnavailable},
		{"the keys fail", ssotest.Keys, ssotest.Fails, entity.ErrOIDCUnavailable},
		{"the keys answer 400", ssotest.Keys, ssotest.Refuses, entity.ErrOIDCUnavailable},
		{"the keys do not come in time", ssotest.Keys, ssotest.Stalls, entity.ErrOIDCUnavailable},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			s, code, state, token := finishElsewhere(t, ssotest.New(t), tt.endpoint, tt.fault)
			if tt.fault == ssotest.Stalls {
				s.SetTimeout(_stall)
			}

			_, err := s.Verify(t.Context(), code, state, token)
			require.ErrorIs(t, err, tt.want)
		})
	}
}

// A caller that goes away while the provider works on its sign-in is neither an outage nor a bad
// sign-in.
func TestACanceledSignInIsNeitherAnOutageNorARefusal(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		endpoint ssotest.Endpoint
	}{
		{"while the provider is discovered", ssotest.Discovery},
		{"while the code is exchanged", ssotest.Token},
		{"while the keys are fetched", ssotest.Keys},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			idp := ssotest.New(t)
			s, code, state, token := finishElsewhere(t, idp, tt.endpoint, ssotest.Stalls)

			// The caller leaves once the provider stalls, past the calls before it.
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()

			go func() {
				select {
				case <-idp.Stalled():
				case <-ctx.Done():
				}

				cancel()
			}()

			_, err := s.Verify(ctx, code, state, token)
			require.ErrorContains(t, err, context.Canceled.Error())
			assert.NotErrorIs(t, err, entity.ErrOIDCUnavailable)
			assert.NotErrorIs(t, err, entity.ErrOIDCCeremonyInvalid)
		})
	}
}
