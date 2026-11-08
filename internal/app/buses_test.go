package app

import (
	"context"
	"log/slog"
	"net/netip"
	"testing"
	"time"

	"github.com/m7medvision/go-clean-template/internal/entity"
	"github.com/m7medvision/go-clean-template/internal/repo/repomock"
	"github.com/m7medvision/go-clean-template/internal/usecase/behavior"
	userquery "github.com/m7medvision/go-clean-template/internal/usecase/user/query"
	"github.com/m7medvision/go-clean-template/pkg/cqrs"
	"github.com/m7medvision/go-clean-template/pkg/postgres"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func testBuses(extra ...cqrs.Step) cqrs.Buses {
	return newBuses(&postgres.Postgres{}, nil, time.Second, prometheus.NewRegistry(), slog.New(slog.DiscardHandler), extra...)
}

func sendSignIn(t *testing.T, b cqrs.Buses, client netip.Addr) (handled bool, err error) {
	t.Helper()

	cqrs.HandleQuery(b.Queries, cqrs.HandlerFunc[userquery.LogIn, entity.SignedIn](
		func(context.Context, userquery.LogIn) (entity.SignedIn, error) {
			handled = true

			return entity.SignedIn{ID: "user-1", Token: "token"}, nil
		}))

	ctx := entity.ContextWithClient(t.Context(), client)
	_, err = cqrs.SendQuery(ctx, b.Queries, userquery.LogIn{Email: "john@example.com", Password: "secret"})

	return handled, err
}

func TestTheAppsBusesRefuseASignInTheLimiterRefuses(t *testing.T) {
	t.Parallel()

	client := netip.MustParseAddr("203.0.113.7")

	lim := repomock.NewMockLimiter(gomock.NewController(t))
	lim.EXPECT().TakeClient(gomock.Any(), entity.LimitSignIn, client).
		Return(entity.Allowance{RetryAfter: time.Minute}, nil)

	handled, err := sendSignIn(t, testBuses(behavior.Limit(lim, slog.New(slog.DiscardHandler))), client)

	require.ErrorIs(t, err, entity.ErrTooManyAttempts)
	assert.False(t, handled)
}

func TestTheWorkersBusesLimitNothing(t *testing.T) {
	t.Parallel()

	handled, err := sendSignIn(t, testBuses(), netip.MustParseAddr("203.0.113.7"))

	require.NoError(t, err)
	assert.True(t, handled)
}
