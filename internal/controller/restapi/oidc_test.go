package restapi_test

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"testing"

	"github.com/m7medvision/go-clean-template/internal/entity"
	"github.com/m7medvision/go-clean-template/internal/usecase"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeOIDC struct {
	usecase.OIDC

	enabled  bool
	authURL  string
	token    string
	err      error
	gotCode  string
	gotState string
	gotToken string
}

func (f *fakeOIDC) Enabled() bool { return f.enabled }

func (f *fakeOIDC) Begin(context.Context) (authURL, token string, err error) {
	return f.authURL, f.token, f.err
}

func (f *fakeOIDC) Finish(_ context.Context, code, state, token string) (string, error) {
	f.gotCode, f.gotState, f.gotToken = code, state, token

	return f.token, f.err
}

func TestOIDCStatus(t *testing.T) {
	t.Parallel()

	for _, enabled := range []bool{true, false} {
		handler := newRouterWithOIDC(newTestConfig(t), &fakeOIDC{enabled: enabled})

		resp := doRequest(t.Context(), handler, http.MethodGet, "/v1/auth/oidc", "", "")

		require.Equal(t, http.StatusOK, resp.Code, "the status is public")

		var body struct {
			Enabled bool `json:"enabled"`
		}

		require.NoError(t, json.Unmarshal(resp.Body.Bytes(), &body))
		assert.Equal(t, enabled, body.Enabled)
	}
}

func TestOIDCBegin(t *testing.T) {
	t.Parallel()

	oc := &fakeOIDC{enabled: true, authURL: "http://idp/auth?state=s", token: "ceremony"}
	handler := newRouterWithOIDC(newTestConfig(t), oc)

	resp := doRequest(t.Context(), handler, http.MethodPost, "/v1/auth/oidc/begin", "", "")

	require.Equal(t, http.StatusOK, resp.Code, resp.Body.String())
	assert.JSONEq(t, `{"url":"http://idp/auth?state=s","ceremony_token":"ceremony"}`, resp.Body.String())
}

func TestOIDCFinish(t *testing.T) {
	t.Parallel()

	oc := &fakeOIDC{enabled: true, token: "app-jwt"}
	handler := newRouterWithOIDC(newTestConfig(t), oc)

	req := `{"code":"the-code","state":"the-state"}`
	resp := doCeremonyRequest(t.Context(), handler, "/v1/auth/oidc/finish", "", "ceremony", req)

	require.Equal(t, http.StatusOK, resp.Code, resp.Body.String())
	assert.JSONEq(t, `{"token":"app-jwt"}`, resp.Body.String())
	assert.Equal(t, "the-code", oc.gotCode)
	assert.Equal(t, "the-state", oc.gotState)
	assert.Equal(t, "ceremony", oc.gotToken)
}

func TestOIDCFinishRejectsEmptyFields(t *testing.T) {
	t.Parallel()

	handler := newRouterWithOIDC(newTestConfig(t), &fakeOIDC{enabled: true})

	resp := doCeremonyRequest(t.Context(), handler, "/v1/auth/oidc/finish", "", "ceremony", `{"code":"","state":"s"}`)

	assert.Equal(t, http.StatusUnprocessableEntity, resp.Code)
}

func TestOIDCErrors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		err  error
		path string
		want int
	}{
		{"begin while disabled", entity.ErrOIDCDisabled, "/v1/auth/oidc/begin", http.StatusNotFound},
		{"begin while the provider is down", entity.ErrOIDCUnavailable, "/v1/auth/oidc/begin", http.StatusBadGateway},
		{"begin failure", io.ErrUnexpectedEOF, "/v1/auth/oidc/begin", http.StatusInternalServerError},
		{"finish while disabled", entity.ErrOIDCDisabled, "/v1/auth/oidc/finish", http.StatusNotFound},
		{"finish with a bad ceremony", entity.ErrOIDCCeremonyInvalid, "/v1/auth/oidc/finish", http.StatusUnauthorized},
		{"finish into a taken email", entity.ErrAccountConflict, "/v1/auth/oidc/finish", http.StatusConflict},
		{"finish while the provider is down", entity.ErrOIDCUnavailable, "/v1/auth/oidc/finish", http.StatusBadGateway},
		{"finish failure", io.ErrUnexpectedEOF, "/v1/auth/oidc/finish", http.StatusInternalServerError},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			handler := newRouterWithOIDC(newTestConfig(t), &fakeOIDC{enabled: true, err: tt.err})

			resp := doCeremonyRequest(t.Context(), handler, tt.path, "", "ceremony", `{"code":"c","state":"s"}`)

			assert.Equal(t, tt.want, resp.Code, resp.Body.String())
			assert.Equal(t, "application/problem+json", resp.Header().Get("Content-Type"))
		})
	}
}
