package oidc_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/m7medvision/go-clean-template/internal/controller/restapi/resttest"
	"github.com/m7medvision/go-clean-template/internal/controller/restapi/v1/oidc"
	"github.com/m7medvision/go-clean-template/internal/entity"
	"github.com/m7medvision/go-clean-template/internal/usecase/oidc/command"
	"github.com/m7medvision/go-clean-template/internal/usecase/oidc/query"
	"github.com/m7medvision/go-clean-template/pkg/cqrs"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeOIDC answers the OIDC command and queries on real buses, and keeps what FinishOIDCLogin was
// sent.
type fakeOIDC struct {
	enabled  bool
	authURL  string
	token    string
	gotCode  string
	gotState string
	gotToken string
}

func (f *fakeOIDC) buses() cqrs.Buses {
	b := resttest.NewBuses()

	cqrs.HandleQuery(b.Queries, cqrs.HandlerFunc[query.GetOIDCStatus, bool](
		func(context.Context, query.GetOIDCStatus) (bool, error) { return f.enabled, nil }))
	cqrs.HandleQuery(b.Queries, cqrs.HandlerFunc[query.BeginOIDCLogin, query.Started](
		func(context.Context, query.BeginOIDCLogin) (query.Started, error) {
			return query.Started{AuthURL: f.authURL, Token: f.token}, nil
		}))
	cqrs.HandleCommand(b.Commands, cqrs.HandlerFunc[command.FinishOIDCLogin, entity.SignedIn](
		func(_ context.Context, c command.FinishOIDCLogin) (entity.SignedIn, error) {
			f.gotCode, f.gotState, f.gotToken = c.Code, c.State, c.Token

			return entity.SignedIn{Token: f.token}, nil
		}))

	return b
}

func newRouterWithOIDC(oc *fakeOIDC) http.Handler {
	return resttest.NewRouter(oc.buses(), oidc.NewRoutes)
}

// postFinish posts body to path with a ceremony token, as the web client posts the callback.
func postFinish(t *testing.T, handler http.Handler, path, body string) *httptest.ResponseRecorder {
	t.Helper()

	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, path, strings.NewReader(body))
	req.Header.Set("X-Ceremony-Token", "ceremony")

	resp := httptest.NewRecorder()
	handler.ServeHTTP(resp, req)

	return resp
}

func TestOIDCStatus(t *testing.T) {
	t.Parallel()

	for _, enabled := range []bool{true, false} {
		handler := newRouterWithOIDC(&fakeOIDC{enabled: enabled})

		resp := resttest.Do(t.Context(), handler, http.MethodGet, "/v1/auth/oidc", "", "")

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
	handler := newRouterWithOIDC(oc)

	resp := resttest.Do(t.Context(), handler, http.MethodPost, "/v1/auth/oidc/begin", "", "")

	require.Equal(t, http.StatusOK, resp.Code, resp.Body.String())
	assert.JSONEq(t, `{"url":"http://idp/auth?state=s","ceremony_token":"ceremony"}`, resp.Body.String())
}

func TestOIDCFinish(t *testing.T) {
	t.Parallel()

	oc := &fakeOIDC{enabled: true, token: "app-jwt"}
	handler := newRouterWithOIDC(oc)

	req := `{"code":"the-code","state":"the-state"}`
	resp := postFinish(t, handler, "/v1/auth/oidc/finish", req)

	require.Equal(t, http.StatusOK, resp.Code, resp.Body.String())
	assert.JSONEq(t, `{"token":"app-jwt"}`, resp.Body.String())
	assert.Equal(t, "the-code", oc.gotCode)
	assert.Equal(t, "the-state", oc.gotState)
	assert.Equal(t, "ceremony", oc.gotToken)
}

func TestOIDCFinishRejectsEmptyFields(t *testing.T) {
	t.Parallel()

	handler := newRouterWithOIDC(&fakeOIDC{enabled: true})

	resp := postFinish(t, handler, "/v1/auth/oidc/finish", `{"code":"","state":"s"}`)

	assert.Equal(t, http.StatusUnprocessableEntity, resp.Code)
}
