// Package graphqltest sends requests to the real GraphQL handler, for the tests of each resolver
// folder: it is to graphql what net/http/httptest is to net/http.
package graphqltest

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/m7medvision/go-clean-template/internal/controller/graphql"
	"github.com/m7medvision/go-clean-template/pkg/cqrs"
	"github.com/m7medvision/go-clean-template/pkg/jwt"
	"github.com/stretchr/testify/require"
)

const (
	// UserID is the caller Token signs for.
	UserID = "user-1"

	_signingKey = "test-key"
)

// Response is a decoded GraphQL response. Body keeps the raw response, so a test can check what it
// leaves out.
type Response struct {
	Data   map[string]json.RawMessage `json:"data"`
	Errors []struct {
		Message    string         `json:"message"`
		Extensions map[string]any `json:"extensions"`
	} `json:"errors"`
	Body string `json:"-"`
}

// Code returns extensions.code of the first error, and fails the test when there is none.
func (r Response) Code(t *testing.T) any {
	t.Helper()
	require.NotEmpty(t, r.Errors)

	return r.Errors[0].Extensions["code"]
}

// Token signs a token for UserID with roles.
func Token(t *testing.T, roles ...string) string {
	t.Helper()

	signed, err := jwt.New(_signingKey, time.Hour).GenerateToken(UserID, roles)
	require.NoError(t, err)

	return signed
}

// Post sends query and vars to a GraphQL handler over the buses b, with token as the bearer token
// unless it is empty. A bus the query never reaches may be nil.
func Post(t *testing.T, b cqrs.Buses, token, query string, vars map[string]any) Response {
	t.Helper()

	payload, err := json.Marshal(map[string]any{"query": query, "variables": vars})
	require.NoError(t, err)

	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/graphql", bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")

	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	rec := httptest.NewRecorder()
	graphql.NewHandler(b, jwt.New(_signingKey, time.Hour), slog.New(slog.DiscardHandler)).ServeHTTP(rec, req)

	var resp Response
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp), rec.Body.String())

	resp.Body = rec.Body.String()

	return resp
}
