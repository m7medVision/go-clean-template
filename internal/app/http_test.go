package app

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/m7medvision/go-clean-template/config"
	"github.com/m7medvision/go-clean-template/internal/entity"
	"github.com/m7medvision/go-clean-template/internal/usecase/task/query"
	"github.com/m7medvision/go-clean-template/pkg/cqrs"
	"github.com/m7medvision/go-clean-template/pkg/jwt"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const _version = "1.0.0"

// newTestHandler serves the app's HTTP mux over b, with metrics on and docs as given, and returns a
// token it accepts.
func newTestHandler(t *testing.T, b cqrs.Buses, docs bool) (handler http.Handler, token string) {
	t.Helper()

	tokens := jwt.New("test-secret", time.Hour)

	token, err := tokens.GenerateToken("user-1", nil)
	require.NoError(t, err)

	cfg := &config.Config{App: config.App{Version: _version}, Metrics: config.Metrics{Enabled: true}, Docs: config.Docs{Enabled: docs}}

	return newHTTPHandler(cfg, b, tokens, slog.New(slog.DiscardHandler)), token
}

func get(t *testing.T, h http.Handler, target, token string) *httptest.ResponseRecorder {
	t.Helper()

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, target, http.NoBody)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	resp := httptest.NewRecorder()
	h.ServeHTTP(resp, req)

	return resp
}

func TestApplianceRoutesArePublic(t *testing.T) {
	t.Parallel()

	h, _ := newTestHandler(t, newTestBuses(), true)

	for _, target := range []string{"/healthz", "/metrics", "/openapi.json", "/docs", "/graphql/playground"} {
		assert.Equal(t, http.StatusOK, get(t, h, target, "").Code, target)
	}
}

func TestATrailingSlashIsStripped(t *testing.T) {
	t.Parallel()

	h, _ := newTestHandler(t, newTestBuses(), false)

	assert.Equal(t, http.StatusOK, get(t, h, "/healthz/", "").Code)
	assert.Equal(t, http.StatusUnauthorized, get(t, h, "/v1/tasks/", "").Code, "the operation answers, not a 404")
}

func TestDocsOffServesNoDocs(t *testing.T) {
	t.Parallel()

	h, _ := newTestHandler(t, newTestBuses(), false)

	for _, target := range []string{"/openapi.json", "/docs", "/graphql/playground"} {
		assert.Equal(t, http.StatusNotFound, get(t, h, target, "").Code, target)
	}
}

// GraphQL is mounted beside REST, not inside it: a request without a token reaches it.
func TestGraphQLIsServedBesideREST(t *testing.T) {
	t.Parallel()

	h, _ := newTestHandler(t, newTestBuses(), false)

	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/graphql", strings.NewReader(`{"query":"{__typename}"}`))
	req.Header.Set("Content-Type", "application/json")

	resp := httptest.NewRecorder()
	h.ServeHTTP(resp, req)

	require.Equal(t, http.StatusOK, resp.Code)
	assert.JSONEq(t, `{"data":{"__typename":"Query"}}`, resp.Body.String())
}

func TestAPanicAnswers500AndIsCounted(t *testing.T) {
	t.Parallel()

	b := newTestBuses()
	cqrs.HandleQuery(b.Queries, cqrs.HandlerFunc[query.GetTask, entity.Task](
		func(context.Context, query.GetTask) (entity.Task, error) { panic("boom") }))

	h, token := newTestHandler(t, b, false)

	resp := get(t, h, "/v1/tasks/task-1", token)
	assert.Equal(t, http.StatusInternalServerError, resp.Code)
	assert.Equal(t, "application/problem+json", resp.Header().Get("Content-Type"))

	metrics := get(t, h, "/metrics", "")
	assert.Contains(t, metrics.Body.String(), `http_requests_total{code="500",method="GET",route="/v1/tasks/{id}"}`)
}

// REST is mounted on the mux, so a request no route matched reaches it. With no operation, there is
// no Security to enforce: it is a 404, token or not.
func TestAnUnmatchedRequestIsCountedAsUnmatched(t *testing.T) {
	t.Parallel()

	h, _ := newTestHandler(t, newTestBuses(), false)

	assert.Equal(t, http.StatusNotFound, get(t, h, "/no-such-page", "").Code)
	assert.Equal(t, http.StatusNotFound, get(t, h, "/v1/no-such-operation", "").Code)

	metrics := get(t, h, "/metrics", "").Body.String()
	assert.Contains(t, metrics, `http_requests_total{code="404",method="GET",route="unmatched"}`)
	assert.NotContains(t, metrics, `route="/*"`)
}

// What the OpenAPI document says needs a token is what the server enforces: every operation that
// declares Security answers a request without a token with 401, and only the sign-in operations
// declare none.
func TestTheDocumentedSecurityIsEnforced(t *testing.T) {
	t.Parallel()

	doc, err := OpenAPI(_version)
	require.NoError(t, err)

	var spec struct {
		Paths map[string]map[string]struct {
			OperationID string                `json:"operationId"`
			Security    []map[string][]string `json:"security"`
		} `json:"paths"`
	}

	require.NoError(t, json.Unmarshal(doc, &spec))

	h, _ := newTestHandler(t, newTestBuses(), false)

	var public []string

	for path, ops := range spec.Paths {
		target := strings.NewReplacer("{id}", "id-1").Replace(path)

		for method, op := range ops {
			req := httptest.NewRequestWithContext(t.Context(), strings.ToUpper(method), target, http.NoBody)
			resp := httptest.NewRecorder()
			h.ServeHTTP(resp, req)

			if len(op.Security) == 0 {
				public = append(public, op.OperationID)

				assert.NotContains(t, resp.Body.String(), "missing authorization header", op.OperationID)

				continue
			}

			assert.Equal(t, http.StatusUnauthorized, resp.Code, op.OperationID)
			assert.JSONEq(t, `{"type":"about:blank","title":"Unauthorized","status":401,"detail":"missing authorization header"}`,
				resp.Body.String(), op.OperationID)
		}
	}

	assert.ElementsMatch(t, []string{
		"register", "login", "begin-passkey-login", "finish-passkey-login", "oidc-status", "begin-oidc", "finish-oidc",
	}, public)
}

// cmd/openapi writes the document the app serves, from the same registration.
func TestTheExportedDocumentIsTheServedOne(t *testing.T) {
	t.Parallel()

	h, _ := newTestHandler(t, newTestBuses(), true)

	resp := get(t, h, "/openapi.json", "")
	require.Equal(t, http.StatusOK, resp.Code)

	exported, err := OpenAPI(_version)
	require.NoError(t, err)

	var served, doc any

	require.NoError(t, json.Unmarshal(resp.Body.Bytes(), &served))
	require.NoError(t, json.Unmarshal(exported, &doc))
	assert.Equal(t, doc, served)
}
