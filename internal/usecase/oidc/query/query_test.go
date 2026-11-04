package query_test

import (
	"net/url"
	"testing"
	"time"

	"github.com/m7medvision/go-clean-template/internal/entity"
	"github.com/m7medvision/go-clean-template/internal/usecase/oidc/oidctest"
	"github.com/m7medvision/go-clean-template/internal/usecase/oidc/query"
	"github.com/m7medvision/go-clean-template/internal/usecase/oidc/sso"
	"github.com/m7medvision/go-clean-template/pkg/jwt"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newSSO(t *testing.T, issuer string) *sso.SSO {
	t.Helper()

	state, err := jwt.NewScoped("test-secret", "oidc-state", time.Minute)
	require.NoError(t, err)

	return sso.New(state, &sso.Config{
		Issuer: issuer, ClientID: oidctest.ClientID, ClientSecret: oidctest.ClientSecret, RedirectURL: oidctest.RedirectURL,
	})
}

func TestBeginOIDCLoginSendsTheBrowserToTheProvider(t *testing.T) {
	t.Parallel()

	idp := oidctest.New(t)

	started, err := query.BeginOIDCLoginHandler{SSO: newSSO(t, idp.Issuer())}.Handle(t.Context(), query.BeginOIDCLogin{})
	require.NoError(t, err)
	require.NotEmpty(t, started.Token)

	u, err := url.Parse(started.AuthURL)
	require.NoError(t, err)
	assert.Equal(t, idp.Issuer()+"/auth", u.Scheme+"://"+u.Host+u.Path)
}

func TestBeginOIDCLoginWithoutAProviderIsDisabled(t *testing.T) {
	t.Parallel()

	_, err := query.BeginOIDCLoginHandler{SSO: newSSO(t, "")}.Handle(t.Context(), query.BeginOIDCLogin{})

	require.ErrorIs(t, err, entity.ErrOIDCDisabled)
}

func TestGetOIDCStatusSaysWhetherAProviderIsConfigured(t *testing.T) {
	t.Parallel()

	on, err := query.GetOIDCStatusHandler{SSO: newSSO(t, "https://sso.example.com")}.Handle(t.Context(), query.GetOIDCStatus{})
	require.NoError(t, err)
	assert.True(t, on)

	off, err := query.GetOIDCStatusHandler{SSO: newSSO(t, "")}.Handle(t.Context(), query.GetOIDCStatus{})
	require.NoError(t, err)
	assert.False(t, off)
}
