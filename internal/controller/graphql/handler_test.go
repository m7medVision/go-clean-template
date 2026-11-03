package graphql_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/m7medvision/go-clean-template/internal/controller/graphql"
	"github.com/m7medvision/go-clean-template/internal/controller/graphql/graphqltest"
	"github.com/m7medvision/go-clean-template/internal/entity"
	"github.com/m7medvision/go-clean-template/internal/usecase"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const _panicDetail = "secret panic detail"

// panicTask is a task use case whose Get panics.
type panicTask struct {
	usecase.Task
}

func (panicTask) Get(context.Context, string, string) (entity.Task, error) {
	panic(_panicDetail)
}

func TestIntrospectionWithoutToken(t *testing.T) {
	t.Parallel()

	resp := graphqltest.Post(t, nil, nil, "", `{ __schema { queryType { name } } }`, nil)

	require.Empty(t, resp.Errors)
	assert.JSONEq(t, `{"queryType":{"name":"Query"}}`, string(resp.Data["__schema"]))
}

func TestPanicIsHidden(t *testing.T) {
	t.Parallel()

	resp := graphqltest.Post(t, panicTask{}, nil, graphqltest.Token(t), `query { task(id: "task-1") { id title status } }`, nil)

	assert.Equal(t, "INTERNAL", resp.Code(t))
	assert.Equal(t, "internal server error", resp.Errors[0].Message)
	assert.NotContains(t, resp.Body, _panicDetail)
}

func TestPlaygroundTargetsEndpoint(t *testing.T) {
	t.Parallel()

	rec := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/graphql/playground", http.NoBody)

	graphql.NewPlayground("/graphql").ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Header().Get("Content-Type"), "text/html")
	assert.Contains(t, rec.Body.String(), `"/graphql"`)
}
