package webapi_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/evrone/go-clean-template/internal/entity"
	"github.com/evrone/go-clean-template/internal/repo/webapi"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The fixture is a recorded api.frankfurter.dev response. The integration-test stub serves the
// same file, so the client and the stub cannot drift apart.
const _fixture = "testdata/frankfurter_latest.json"

func newServer(t *testing.T, status int, body []byte) *httptest.Server {
	t.Helper()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/latest" || r.URL.Query().Get("base") != "EUR" || r.URL.Query().Get("symbols") != "USD" {
			w.WriteHeader(http.StatusBadRequest)

			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)

		if _, err := w.Write(body); err != nil {
			t.Error(err)
		}
	}))
	t.Cleanup(srv.Close)

	return srv
}

func TestExchangeWebAPI_Rate(t *testing.T) {
	t.Parallel()

	fixture, err := os.ReadFile(_fixture)
	require.NoError(t, err)

	t.Run("reads the rate and its date from the fixture", func(t *testing.T) {
		t.Parallel()

		srv := newServer(t, http.StatusOK, fixture)

		rate, date, err := webapi.New(srv.URL).Rate(t.Context(), "EUR", "USD")

		require.NoError(t, err)
		assert.True(t, rate.Equal(decimal.RequireFromString("1.1225")), rate.String())
		assert.Equal(t, time.Date(2026, 10, 19, 0, 0, 0, 0, time.UTC), date)
	})

	t.Run("404 means an unsupported currency", func(t *testing.T) {
		t.Parallel()

		srv := newServer(t, http.StatusNotFound, []byte(`{"message":"not found"}`))

		_, _, err := webapi.New(srv.URL).Rate(t.Context(), "EUR", "USD")

		require.ErrorIs(t, err, entity.ErrUnsupportedCurrency)
	})

	t.Run("a missing target rate means an unsupported currency", func(t *testing.T) {
		t.Parallel()

		srv := newServer(t, http.StatusOK, []byte(`{"amount":1.0,"base":"EUR","date":"2026-10-19","rates":{}}`))

		_, _, err := webapi.New(srv.URL).Rate(t.Context(), "EUR", "USD")

		require.ErrorIs(t, err, entity.ErrUnsupportedCurrency)
	})

	t.Run("a server error is an error, not an unsupported currency", func(t *testing.T) {
		t.Parallel()

		srv := newServer(t, http.StatusInternalServerError, nil)

		_, _, err := webapi.New(srv.URL).Rate(t.Context(), "EUR", "USD")

		require.Error(t, err)
		assert.NotErrorIs(t, err, entity.ErrUnsupportedCurrency)
	})

	t.Run("a canceled context stops the call", func(t *testing.T) {
		t.Parallel()

		srv := newServer(t, http.StatusOK, fixture)

		ctx, cancel := context.WithCancel(t.Context())
		cancel()

		_, _, err := webapi.New(srv.URL).Rate(ctx, "EUR", "USD")

		require.ErrorIs(t, err, context.Canceled)
	})
}
