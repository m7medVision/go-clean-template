// Package oidc signs users in through an OpenID Connect provider.
package oidc

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
	"github.com/google/uuid"
	"github.com/m7medvision/go-clean-template/internal/entity"
	"github.com/m7medvision/go-clean-template/internal/repo"
	"github.com/m7medvision/go-clean-template/internal/usecase"
	"github.com/m7medvision/go-clean-template/pkg/jwt"
	"golang.org/x/oauth2"
)

// Config -.
type Config struct {
	// Issuer is the provider's issuer URL; empty turns SSO off.
	Issuer       string
	ClientID     string
	ClientSecret string
	// RedirectURL is where the provider sends the browser back: the web client's callback route.
	RedirectURL string
	// RolesClaim is the dotted path to the ID-token claim holding the provider's role names.
	RolesClaim string
	// RoleMap maps provider role names to app role names.
	RoleMap map[string]string
}

// ceremony is the state a sign-in carries from Begin to Finish inside the signed token.
type ceremony struct {
	State    string `json:"s"`
	Nonce    string `json:"n"`
	Verifier string `json:"v"`
}

// UseCase -.
type UseCase struct {
	identities repo.IdentityRepo
	state      *jwt.Scoped
	tokens     *jwt.Manager
	cfg        Config

	// mu guards provider, discovered on the first sign-in rather than at startup, so the app starts
	// while the provider is still down. A failed discovery is retried on the next sign-in.
	mu       sync.Mutex
	provider *gooidc.Provider
}

// New returns an OIDC use case instrumented with OpenTelemetry tracing spans. state signs the
// ceremony token; its lifetime bounds how long a sign-in may take at the provider. tokens issues the
// auth token Finish returns.
func New(identities repo.IdentityRepo, state *jwt.Scoped, tokens *jwt.Manager, cfg *Config) usecase.OIDC {
	return newTraced(&UseCase{identities: identities, state: state, tokens: tokens, cfg: *cfg})
}

// Enabled -.
func (uc *UseCase) Enabled() bool {
	return uc.cfg.Issuer != ""
}

// Begin -.
func (uc *UseCase) Begin(ctx context.Context) (authURL, token string, err error) {
	if !uc.Enabled() {
		return "", "", entity.ErrOIDCDisabled
	}

	provider, err := uc.discover(ctx)
	if err != nil {
		return "", "", err
	}

	c := ceremony{State: rand.Text(), Nonce: rand.Text(), Verifier: oauth2.GenerateVerifier()}

	payload, err := json.Marshal(c)
	if err != nil {
		return "", "", fmt.Errorf("OIDCUseCase - Begin - json.Marshal: %w", err)
	}

	token, err = uc.state.Sign(payload)
	if err != nil {
		return "", "", fmt.Errorf("OIDCUseCase - Begin - uc.state.Sign: %w", err)
	}

	authURL = uc.oauth(provider).AuthCodeURL(c.State, gooidc.Nonce(c.Nonce), oauth2.S256ChallengeOption(c.Verifier))

	return authURL, token, nil
}

// Finish -.
func (uc *UseCase) Finish(ctx context.Context, code, state, token string) (string, error) {
	if !uc.Enabled() {
		return "", entity.ErrOIDCDisabled
	}

	c, err := uc.verifyCeremony(token, state)
	if err != nil {
		return "", err
	}

	provider, err := uc.discover(ctx)
	if err != nil {
		return "", err
	}

	idToken, err := uc.exchange(ctx, provider, code, c)
	if err != nil {
		return "", err
	}

	var claims map[string]any
	if err = idToken.Claims(&claims); err != nil {
		return "", fmt.Errorf("OIDCUseCase - Finish - idToken.Claims: %w", err)
	}

	user, err := uc.signInUser(ctx, idToken, claims)
	if err != nil {
		return "", err
	}

	authToken, err := uc.tokens.GenerateToken(user.ID, entity.RoleNames(uc.roles(claims)))
	if err != nil {
		return "", fmt.Errorf("OIDCUseCase - Finish - uc.tokens.GenerateToken: %w", err)
	}

	return authToken, nil
}

func (uc *UseCase) discover(ctx context.Context) (*gooidc.Provider, error) {
	uc.mu.Lock()
	defer uc.mu.Unlock()

	if uc.provider != nil {
		return uc.provider, nil
	}

	provider, err := gooidc.NewProvider(ctx, uc.cfg.Issuer)
	if err != nil {
		return nil, fmt.Errorf("OIDCUseCase - discover - oidc.NewProvider: %w: %w", entity.ErrOIDCUnavailable, err)
	}

	uc.provider = provider

	return provider, nil
}

func (uc *UseCase) oauth(provider *gooidc.Provider) *oauth2.Config {
	return &oauth2.Config{
		ClientID:     uc.cfg.ClientID,
		ClientSecret: uc.cfg.ClientSecret,
		Endpoint:     provider.Endpoint(),
		RedirectURL:  uc.cfg.RedirectURL,
		Scopes:       []string{gooidc.ScopeOpenID, "email", "profile"},
	}
}

func (uc *UseCase) verifyCeremony(token, state string) (ceremony, error) {
	payload, err := uc.state.Verify(token)
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
func (uc *UseCase) exchange(ctx context.Context, provider *gooidc.Provider, code string, c ceremony) (*gooidc.IDToken, error) {
	oauthToken, err := uc.oauth(provider).Exchange(ctx, code, oauth2.VerifierOption(c.Verifier))
	if err != nil {
		// The provider answered and refused the code: unknown, used, expired, or another verifier.
		if rejected, ok := errors.AsType[*oauth2.RetrieveError](err); ok && rejected != nil {
			return nil, entity.ErrOIDCCeremonyInvalid
		}

		return nil, fmt.Errorf("OIDCUseCase - exchange - Exchange: %w", err)
	}

	rawIDToken, ok := oauthToken.Extra("id_token").(string)
	if !ok {
		return nil, entity.ErrOIDCCeremonyInvalid
	}

	idToken, err := provider.Verifier(&gooidc.Config{ClientID: uc.cfg.ClientID}).Verify(ctx, rawIDToken)
	if err != nil {
		return nil, entity.ErrOIDCCeremonyInvalid
	}

	if subtle.ConstantTimeCompare([]byte(idToken.Nonce), []byte(c.Nonce)) != 1 {
		return nil, entity.ErrOIDCCeremonyInvalid
	}

	return idToken, nil
}

// signInUser returns the user linked to the ID token's identity, creating one on its first sign-in.
func (uc *UseCase) signInUser(ctx context.Context, idToken *gooidc.IDToken, claims map[string]any) (entity.User, error) {
	email, ok := claims["email"].(string)
	if !ok || email == "" {
		return entity.User{}, entity.ErrOIDCCeremonyInvalid
	}

	user, err := uc.identities.UserByIdentity(ctx, idToken.Issuer, idToken.Subject)
	if err == nil {
		return user, nil
	}

	if !errors.Is(err, entity.ErrUserNotFound) {
		return entity.User{}, fmt.Errorf("OIDCUseCase - signInUser - uc.identities.UserByIdentity: %w", err)
	}

	username, ok := claims["preferred_username"].(string)
	if !ok || username == "" {
		username = email
	}

	user = entity.User{ID: uuid.NewString(), Username: username, Email: email, CreatedAt: time.Now()}
	user.UpdatedAt = user.CreatedAt

	err = uc.identities.CreateWithIdentity(ctx, &user, idToken.Issuer, idToken.Subject)
	if errors.Is(err, entity.ErrUserAlreadyExists) {
		// A concurrent first sign-in of the same identity linked it first.
		user, err = uc.identities.UserByIdentity(ctx, idToken.Issuer, idToken.Subject)
	}

	if err != nil {
		if errors.Is(err, entity.ErrAccountConflict) {
			return entity.User{}, err
		}

		return entity.User{}, fmt.Errorf("OIDCUseCase - signInUser - uc.identities.CreateWithIdentity: %w", err)
	}

	return user, nil
}

// roles maps the provider role names at cfg.RolesClaim through cfg.RoleMap. A sign-in that maps to
// no app role gets the local roles.
func (uc *UseCase) roles(claims map[string]any) []entity.Role {
	var names []string

	for _, name := range claimStrings(claims, uc.cfg.RolesClaim) {
		if role, ok := uc.cfg.RoleMap[name]; ok {
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
