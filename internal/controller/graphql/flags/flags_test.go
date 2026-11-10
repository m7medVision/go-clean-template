package flags_test

import (
	"context"
	"testing"

	"github.com/m7medvision/go-clean-template/internal/controller/graphql/graphqltest"
	"github.com/m7medvision/go-clean-template/internal/entity"
	"github.com/m7medvision/go-clean-template/internal/usecase/flags/query"
	"github.com/m7medvision/go-clean-template/pkg/cqrs"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const _flags = `query { flags { taskAttributes } }`

func buses(gotUser *string) cqrs.Buses {
	qs := cqrs.NewQueries()
	cqrs.HandleQuery(qs, cqrs.HandlerFunc[query.GetFlags, entity.FlagValues](
		func(_ context.Context, q query.GetFlags) (entity.FlagValues, error) {
			*gotUser = q.UserID

			return entity.FlagValues{TaskAttributes: true}, nil
		}))

	return cqrs.Buses{Commands: cqrs.NewCommands(), Queries: qs}
}

func TestFlags(t *testing.T) {
	t.Parallel()

	var gotUser string

	resp := graphqltest.Post(t, buses(&gotUser), graphqltest.Token(t), _flags, nil)

	require.Empty(t, resp.Errors)
	assert.JSONEq(t, `{"taskAttributes":true}`, string(resp.Data["flags"]))
	assert.Equal(t, graphqltest.UserID, gotUser)
}

func TestFlagsUnauthenticated(t *testing.T) {
	t.Parallel()

	var gotUser string

	resp := graphqltest.Post(t, buses(&gotUser), "", _flags, nil)

	assert.Equal(t, "UNAUTHENTICATED", resp.Code(t))
	assert.Empty(t, gotUser)
}
