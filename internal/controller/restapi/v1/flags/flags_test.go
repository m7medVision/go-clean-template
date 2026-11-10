package flags_test

import (
	"context"
	"net/http"
	"testing"

	"github.com/m7medvision/go-clean-template/internal/controller/restapi/resttest"
	"github.com/m7medvision/go-clean-template/internal/controller/restapi/v1/flags"
	"github.com/m7medvision/go-clean-template/internal/entity"
	"github.com/m7medvision/go-clean-template/internal/usecase/flags/query"
	"github.com/m7medvision/go-clean-template/pkg/cqrs"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetFlags(t *testing.T) {
	t.Parallel()

	var gotUser string

	b := resttest.NewBuses()
	cqrs.HandleQuery(b.Queries, cqrs.HandlerFunc[query.GetFlags, entity.FlagValues](
		func(_ context.Context, q query.GetFlags) (entity.FlagValues, error) {
			gotUser = q.UserID

			return entity.FlagValues{TaskAttributes: true}, nil
		}))
	handler := resttest.NewRouter(b, flags.NewRoutes)

	resp := resttest.Do(t.Context(), handler, http.MethodGet, "/v1/flags", resttest.Token(t), "")

	require.Equal(t, http.StatusOK, resp.Code, resp.Body.String())
	assert.JSONEq(t, `{"taskAttributes":true}`, resp.Body.String())
	assert.NotEmpty(t, gotUser)

	resp = resttest.Do(t.Context(), handler, http.MethodGet, "/v1/flags", "", "")
	assert.Equal(t, http.StatusUnauthorized, resp.Code)
}
