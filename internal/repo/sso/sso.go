// Package sso runs the OpenID Connect sign-in ceremony (authorization code with PKCE) against the
// configured provider. Begin returns the provider URL to send the browser to and a token carrying
// the ceremony's state; the provider redirects back with a code and the state, which Verify checks
// against the token before it exchanges the code and verifies the ID token. It stores nothing: the
// OIDC commands decide which user an identity is.
//
// Every call to the provider follows one rule. The provider turning the code down (a 4xx), or an ID
// token that does not verify, is entity.ErrOIDCCeremonyInvalid. Failing to reach the provider or to
// get an answer from it (a transport error, Timeout, a 5xx, keys that cannot be fetched) is
// entity.ErrOIDCUnavailable. A caller that went away is neither.
package sso

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"slices"
	"strings"
	"sync"
	"time"

	gooidc "github.com/coreos/go-oidc/v3/oidc"
	"github.com/m7medvision/go-clean-template/config"
	"github.com/m7medvision/go-clean-template/internal/entity"
	"github.com/m7medvision/go-clean-template/internal/repo"
	"github.com/m7medvision/go-clean-template/pkg/jwt"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"golang.org/x/oauth2"
)

// Timeout bounds the calls one step of a sign-in (Begin or Verify) makes to the provider, all of them
// together. The app's HTTP write deadline leaves room for it, since finishing a sign-in waits for the
// provider before it is saved.
const Timeout = 5 * time.Second

const (
	// _stateScope scopes the sign-in state token's key and audience, so no other token passes for it.
	_stateScope = "oidc-state"
	// _stateTimeout is how long a user has to sign in at the provider.
	_stateTimeout = 10 * time.Minute
	// _keysUnfetched is in the error go-oidc's Verify returns when it could not fetch the provider's
	// keys. Verify formats the key set's error into its own text, so the text is all that tells keys
	// it could not fetch from a token no key verifies. Should go-oidc reword it, a key fetch outage
	// falls back to entity.ErrOIDCCeremonyInvalid, never the reverse, and the key tests fail.
	_keysUnfetched = "fetching keys"
)

// ceremony is the state a sign-in carries from Begin to Verify inside the signed token.
type ceremony struct {
	State    string `json:"s"`
	Nonce    string `json:"n"`
	Verifier string `json:"v"`
}

// ErrUnknownRole is an OIDC_ROLE_MAP pair whose app role the app does not have.
var ErrUnknownRole = errors.New("sso: OIDC_ROLE_MAP names a role the app does not have")

// errTimedOut ends a step whose calls to the provider outlast its timeout. It tells the provider
// being slow from the caller going away.
var errTimedOut = errors.New("sso: the provider did not answer in time")

type SSO struct {
	state *jwt.Scoped
	cfg   config.OIDC
	// roles is cfg.RoleMap with its values parsed: provider role name to app role.
	roles map[string]entity.Role
	// redirectURL is where the provider sends the browser back to.
	redirectURL string
	// client makes every call to the provider. Its own timeout bounds the key fetches go-oidc runs in
	// the background, which no step's deadline reaches.
	client *http.Client
	// timeout bounds the calls one step makes to the provider: Timeout, shorter in tests.
	timeout time.Duration

	// mu guards provider, discovered on the first sign-in rather than at startup, so the app starts
	// while the provider is still down. A failed discovery is retried on the next sign-in.
	mu       sync.Mutex
	provider *gooidc.Provider
}

var _ repo.SSOProvider = (*SSO)(nil)

// New returns the sign-in at cfg's provider, which sends the browser back to redirectURL (the web
// client's callback, config.App.SSOCallbackURL). Its state token is signed with a key derived from
// secret. The provider is discovered on the first sign-in, so New never fails on it. An empty
// cfg.Issuer turns SSO off. A cfg.RoleMap value that is no app role is ErrUnknownRole, so a typo
// stops the app at startup instead of granting nothing on every sign-in. Its calls to the provider go
// through a transport that opens an OpenTelemetry client span per request.
func New(secret string, cfg config.OIDC, redirectURL string) (*SSO, error) {
	roles, err := parseRoleMap(cfg.RoleMap)
	if err != nil {
		return nil, fmt.Errorf("sso - New: %w", err)
	}

	state, err := jwt.NewScoped(secret, _stateScope, _stateTimeout)
	if err != nil {
		return nil, fmt.Errorf("sso - New - jwt.NewScoped: %w", err)
	}

	return &SSO{
		state: state, cfg: cfg, roles: roles, redirectURL: redirectURL,
		client:  &http.Client{Timeout: Timeout, Transport: otelhttp.NewTransport(http.DefaultTransport)},
		timeout: Timeout,
	}, nil
}

// parseRoleMap turns each provider-name:app-role pair into an app role, refusing a role the app does
// not have.
func parseRoleMap(pairs map[string]string) (map[string]entity.Role, error) {
	roles := make(map[string]entity.Role, len(pairs))

	for name, value := range pairs {
		role := entity.Role(value)
		if !role.Valid() {
			return nil, fmt.Errorf("%w: %s:%s", ErrUnknownRole, name, value)
		}

		roles[name] = role
	}

	return roles, nil
}

// Enabled reports whether a provider is configured.
func (s *SSO) Enabled() bool {
	return s.cfg.Issuer != ""
}

// Begin returns the provider URL to send the browser to and the token Verify needs. With no
// provider configured it is entity.ErrOIDCDisabled; with the provider unreachable,
// entity.ErrOIDCUnavailable.
func (s *SSO) Begin(ctx context.Context) (authURL, token string, err error) {
	if !s.Enabled() {
		return "", "", entity.ErrOIDCDisabled
	}

	ctx, cancel := s.call(ctx)
	defer cancel()

	provider, err := s.discover(ctx)
	if err != nil {
		return "", "", err
	}

	c := ceremony{State: rand.Text(), Nonce: rand.Text(), Verifier: oauth2.GenerateVerifier()}

	payload, err := json.Marshal(c)
	if err != nil {
		return "", "", fmt.Errorf("sso - Begin - json.Marshal: %w", err)
	}

	token, err = s.state.Sign(payload)
	if err != nil {
		return "", "", fmt.Errorf("sso - Begin - s.state.Sign: %w", err)
	}

	authURL = s.oauth(provider).AuthCodeURL(c.State, gooidc.Nonce(c.Nonce), oauth2.S256ChallengeOption(c.Verifier))

	return authURL, token, nil
}

// Verify checks the provider's redirect (code and state) against the ceremony in token, exchanges
// the code, verifies the ID token, and returns who signed in. A bad, expired or replayed state token,
// code or ID token, or an ID token without an email, is entity.ErrOIDCCeremonyInvalid. A provider
// that cannot be reached at any of these steps (discovery, the exchange, the key fetch) is
// entity.ErrOIDCUnavailable.
func (s *SSO) Verify(ctx context.Context, code, state, token string) (entity.SSOIdentity, error) {
	if !s.Enabled() {
		return entity.SSOIdentity{}, entity.ErrOIDCDisabled
	}

	c, err := s.verifyCeremony(token, state)
	if err != nil {
		return entity.SSOIdentity{}, err
	}

	ctx, cancel := s.call(ctx)
	defer cancel()

	provider, err := s.discover(ctx)
	if err != nil {
		return entity.SSOIdentity{}, err
	}

	idToken, err := s.exchange(ctx, provider, code, c)
	if err != nil {
		return entity.SSOIdentity{}, err
	}

	var claims map[string]any
	if err = idToken.Claims(&claims); err != nil {
		return entity.SSOIdentity{}, fmt.Errorf("sso - Verify - idToken.Claims: %w", err)
	}

	email, ok := claims["email"].(string)
	if !ok || email == "" {
		return entity.SSOIdentity{}, entity.ErrOIDCCeremonyInvalid
	}

	username, ok := claims["preferred_username"].(string)
	if !ok || username == "" {
		username = email
	}

	return entity.SSOIdentity{
		Issuer: idToken.Issuer, Subject: idToken.Subject, Email: email, Username: username, Roles: s.rolesOf(claims),
	}, nil
}

// call is ctx for the calls one step makes to the provider: bounded by the timeout, all of them
// together, and carrying the client, which go-oidc and oauth2 both read from it. The provider keeps
// the client it was discovered with for its key fetches.
func (s *SSO) call(ctx context.Context) (context.Context, context.CancelFunc) {
	ctx, cancel := context.WithTimeoutCause(ctx, s.timeout, errTimedOut)

	return gooidc.ClientContext(ctx, s.client), cancel
}

// unavailable is err, from op's call to the provider, as entity.ErrOIDCUnavailable, unless the caller
// went away first: a canceled request is not the provider's failure. ctx is the step's, from call,
// whose cause tells its own timeout from the caller's end.
func unavailable(ctx context.Context, op string, err error) error {
	if ctx.Err() != nil && !errors.Is(context.Cause(ctx), errTimedOut) {
		return fmt.Errorf("%s: %w", op, err)
	}

	return fmt.Errorf("%s: %w: %w", op, entity.ErrOIDCUnavailable, err)
}

func (s *SSO) discover(ctx context.Context) (*gooidc.Provider, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.provider != nil {
		return s.provider, nil
	}

	provider, err := gooidc.NewProvider(ctx, s.cfg.Issuer)
	if err != nil {
		return nil, unavailable(ctx, "sso - discover - oidc.NewProvider", err)
	}

	s.provider = provider

	return provider, nil
}

func (s *SSO) oauth(provider *gooidc.Provider) *oauth2.Config {
	return &oauth2.Config{
		ClientID:     s.cfg.ClientID,
		ClientSecret: s.cfg.ClientSecret,
		Endpoint:     provider.Endpoint(),
		RedirectURL:  s.redirectURL,
		Scopes:       []string{gooidc.ScopeOpenID, "email", "profile"},
	}
}

func (s *SSO) verifyCeremony(token, state string) (ceremony, error) {
	payload, err := s.state.Verify(token)
	if err != nil {
		return ceremony{}, entity.ErrOIDCCeremonyInvalid
	}

	var c ceremony
	if err = json.Unmarshal(payload, &c); err != nil {
		return ceremony{}, entity.ErrOIDCCeremonyInvalid
	}

	if c.State == "" || subtle.ConstantTimeCompare([]byte(c.State), []byte(state)) != 1 {
		return ceremony{}, entity.ErrOIDCCeremonyInvalid
	}

	return c, nil
}

// exchange trades the code for tokens and verifies the ID token: signature, issuer, audience,
// expiry, and the nonce this sign-in sent.
func (s *SSO) exchange(ctx context.Context, provider *gooidc.Provider, code string, c ceremony) (*gooidc.IDToken, error) {
	oauthToken, err := s.oauth(provider).Exchange(ctx, code, oauth2.VerifierOption(c.Verifier))
	if err != nil {
		if refused(err) {
			return nil, entity.ErrOIDCCeremonyInvalid
		}

		return nil, unavailable(ctx, "sso - exchange - Exchange", err)
	}

	rawIDToken, ok := oauthToken.Extra("id_token").(string)
	if !ok {
		return nil, entity.ErrOIDCCeremonyInvalid
	}

	idToken, err := provider.Verifier(&gooidc.Config{ClientID: s.cfg.ClientID}).Verify(ctx, rawIDToken)
	if err != nil {
		// Verify fetches the keys when none it holds verifies the token; any other failure is the token's.
		if strings.Contains(err.Error(), _keysUnfetched) {
			return nil, unavailable(ctx, "sso - exchange - Verify", err)
		}

		return nil, entity.ErrOIDCCeremonyInvalid
	}

	if subtle.ConstantTimeCompare([]byte(idToken.Nonce), []byte(c.Nonce)) != 1 {
		return nil, entity.ErrOIDCCeremonyInvalid
	}

	return idToken, nil
}

// refused reports whether err is the provider answering the exchange and turning the code down
// (unknown, used, expired, or sent with another verifier): an OAuth error with a 4xx, or with the 200
// some providers send it with. A 5xx is the provider failing.
func refused(err error) bool {
	rejected, ok := errors.AsType[*oauth2.RetrieveError](err)

	return ok && rejected != nil && rejected.Response != nil &&
		rejected.Response.StatusCode < http.StatusInternalServerError
}

// rolesOf maps the provider role names at cfg.RolesClaim through the role map, in order and without
// duplicates. A sign-in that maps to no app role gets the local roles.
func (s *SSO) rolesOf(claims map[string]any) []entity.Role {
	var roles []entity.Role

	for _, name := range claimStrings(claims, s.cfg.RolesClaim) {
		if role, ok := s.roles[name]; ok && !slices.Contains(roles, role) {
			roles = append(roles, role)
		}
	}

	if len(roles) == 0 {
		return entity.LocalRoles()
	}

	return roles
}

// claimStrings reads the string or string array at a dotted path in claims; anything else is empty.
func claimStrings(claims map[string]any, path string) []string {
	var value any = claims

	for key := range strings.SplitSeq(path, ".") {
		object, ok := value.(map[string]any)
		if !ok {
			return nil
		}

		value = object[key]
	}

	switch v := value.(type) {
	case string:
		return []string{v}
	case []any:
		names := make([]string, 0, len(v))

		for _, item := range v {
			if name, ok := item.(string); ok {
				names = append(names, name)
			}
		}

		return names
	default:
		return nil
	}
}
