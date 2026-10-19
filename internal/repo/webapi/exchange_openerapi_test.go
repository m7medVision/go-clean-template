package webapi_test

import (
	"context"
	"errors"
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

// The fixture is a recorded open.er-api.com response for OMR. The integration-test stub serves the
// same file, so the client and the stub cannot drift apart.
const _fixture = "testdata/openerapi_latest_omr.json"

func newServer(t *testing.T, status int, body []byte) *httptest.Server {
	t.Helper()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v6/latest/OMR" {
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

		rate, date, err := webapi.New(srv.URL).Rate(t.Context(), "OMR", "CNY")

		require.NoError(t, err)
		assert.True(t, rate.Equal(decimal.RequireFromString("17.406483")), rate.String())
		assert.Equal(t, time.Date(2026, 10, 19, 0, 0, 0, 0, time.UTC), date)
	})

	t.Run("a server error is an error, not an unsupported currency", func(t *testing.T) {
		t.Parallel()

		srv := newServer(t, http.StatusInternalServerError, nil)

		_, _, err := webapi.New(srv.URL).Rate(t.Context(), "OMR", "CNY")

		require.Error(t, err)
		assert.NotErrorIs(t, err, entity.ErrUnsupportedCurrency)
	})

	t.Run("a canceled context stops the call", func(t *testing.T) {
		t.Parallel()

		srv := newServer(t, http.StatusOK, fixture)

		ctx, cancel := context.WithCancel(t.Context())
		cancel()

		_, _, err := webapi.New(srv.URL).Rate(ctx, "OMR", "CNY")

		require.ErrorIs(t, err, context.Canceled)
	})
}

func TestExchangeWebAPI_RateErrorBodies(t *testing.T) {
	t.Parallel()

	// open.er-api.com reports errors with HTTP 200, so the client reads the body.
	tests := []struct {
		name            string
		body            string
		wantUnsupported bool
	}{
		{"unsupported-code means an unsupported currency", `{"result":"error","error-type":"unsupported-code"}`, true},
		{
			"a missing target rate means an unsupported currency",
			`{"result":"success","base_code":"OMR","time_last_update_unix":1792368152,"rates":{}}`,
			true,
		},
		{"any other error is an error, not an unsupported currency", `{"result":"error","error-type":"rate-limited"}`, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			srv := newServer(t, http.StatusOK, []byte(tt.body))

			_, _, err := webapi.New(srv.URL).Rate(t.Context(), "OMR", "CNY")

			require.Error(t, err)
			assert.Equal(t, tt.wantUnsupported, errors.Is(err, entity.ErrUnsupportedCurrency))
		})
	}
}
