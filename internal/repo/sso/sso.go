// Package sso runs the OpenID Connect sign-in ceremony (authorization code with PKCE) against the
// configured provider. Begin returns the provider URL to send the browser to and a token carrying
// the ceremony's state; the provider redirects back with a code and the state, which Verify checks
// against the token before it exchanges the code and verifies the ID token. It stores nothing: the
// OIDC commands decide which user an identity is.
package sso

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	gooidc "github.com/coreos/go-oidc/v3/oidc"
	"github.com/m7medvision/go-clean-template/config"
	"github.com/m7medvision/go-clean-template/internal/entity"
	"github.com/m7medvision/go-clean-template/internal/repo"
	"github.com/m7medvision/go-clean-template/pkg/jwt"
	"golang.org/x/oauth2"
)

const (
	// _stateScope scopes the sign-in state token's key and audience, so no other token passes for it.
	_stateScope = "oidc-state"
	// _stateTimeout is how long a user has to sign in at the provider.
	_stateTimeout = 10 * time.Minute
	// _callbackPath is the web client's route the provider redirects the browser back to.
	_callbackPath = "/sso/callback"
)

// ceremony is the state a sign-in carries from Begin to Verify inside the signed token.
type ceremony struct {
	State    string `json:"s"`
	Nonce    string `json:"n"`
	Verifier string `json:"v"`
}

// SSO signs users in at the provider.
type SSO struct {
	state *jwt.Scoped
	cfg   config.OIDC
	// redirectURL is where the provider sends the browser back to.
	redirectURL string

	// mu guards provider, discovered on the first sign-in rather than at startup, so the app starts
	// while the provider is still down. A failed discovery is retried on the next sign-in.
	mu       sync.Mutex
	provider *gooidc.Provider
}

var _ repo.SSOProvider = (*SSO)(nil)

// New returns the sign-in at cfg's provider, which redirects back to the app at baseURL. Its
// state token is signed with a key derived from secret. The provider is discovered on the first
// sign-in, so New never fails on it. An empty cfg.Issuer turns SSO off.
func New(secret string, cfg config.OIDC, baseURL string) (*SSO, error) {
	state, err := jwt.NewScoped(secret, _stateScope, _stateTimeout)
	if err != nil {
		return nil, fmt.Errorf("sso - New - jwt.NewScoped: %w", err)
	}

	return &SSO{state: state, cfg: cfg, redirectURL: baseURL + _callbackPath}, nil
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
// code or ID token, or an ID token without an email, is entity.ErrOIDCCeremonyInvalid.
func (s *SSO) Verify(ctx context.Context, code, state, token string) (entity.SSOIdentity, error) {
	if !s.Enabled() {
		return entity.SSOIdentity{}, entity.ErrOIDCDisabled
	}

	c, err := s.verifyCeremony(token, state)
	if err != nil {
		return entity.SSOIdentity{}, err
	}

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
		Issuer: idToken.Issuer, Subject: idToken.Subject, Email: email, Username: username, Roles: s.roles(claims),
	}, nil
}

func (s *SSO) discover(ctx context.Context) (*gooidc.Provider, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.provider != nil {
		return s.provider, nil
	}

	provider, err := gooidc.NewProvider(ctx, s.cfg.Issuer)
	if err != nil {
		return nil, fmt.Errorf("sso - discover - oidc.NewProvider: %w: %w", entity.ErrOIDCUnavailable, err)
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
		// The provider answered and refused the code: unknown, used, expired, or another verifier.
		if rejected, ok := errors.AsType[*oauth2.RetrieveError](err); ok && rejected != nil {
			return nil, entity.ErrOIDCCeremonyInvalid
		}

		return nil, fmt.Errorf("sso - exchange - Exchange: %w", err)
	}

	rawIDToken, ok := oauthToken.Extra("id_token").(string)
	if !ok {
		return nil, entity.ErrOIDCCeremonyInvalid
	}

	idToken, err := provider.Verifier(&gooidc.Config{ClientID: s.cfg.ClientID}).Verify(ctx, rawIDToken)
	if err != nil {
		return nil, entity.ErrOIDCCeremonyInvalid
	}

	if subtle.ConstantTimeCompare([]byte(idToken.Nonce), []byte(c.Nonce)) != 1 {
		return nil, entity.ErrOIDCCeremonyInvalid
	}

	return idToken, nil
}

// roles maps the provider role names at cfg.RolesClaim through cfg.RoleMap. A sign-in that maps to
// no app role gets the local roles.
func (s *SSO) roles(claims map[string]any) []entity.Role {
	var names []string

	for _, name := range claimStrings(claims, s.cfg.RolesClaim) {
		if role, ok := s.cfg.RoleMap[name]; ok {
			names = append(names, role)
		}
	}

	roles := entity.ParseRoles(names)
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
