package webapi_test

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/m7medvision/go-clean-template/internal/entity"
	"github.com/m7medvision/go-clean-template/internal/repo/webapi"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The fixture is a recorded open.er-api.com response for OMR. The integration-test stub serves the
// same file, so the client and the stub cannot drift apart.
const _fixture = "testdata/openerapi_latest_omr.json"

const (
	_cacheKey     = "exchange:latest:OMR"
	_maxBody      = 1 << 20
	_freshRate    = "17.5"
	_shortTimeout = 100 * time.Millisecond
)

var errCacheDown = errors.New("cache down")

type entry struct {
	value     []byte
	expiresAt time.Time
}

type fakeCache struct {
	failGet, hangGet, failSet bool

	mu      sync.Mutex
	entries map[string]entry
}

func (c *fakeCache) Get(ctx context.Context, key string) (value []byte, found bool, err error) {
	if c.hangGet {
		<-ctx.Done()

		return nil, false, ctx.Err()
	}

	if c.failGet {
		return nil, false, errCacheDown
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	e, ok := c.entries[key]

	return e.value, ok, nil
}

func (c *fakeCache) Set(_ context.Context, key string, value []byte, expiresAt time.Time) error {
	if c.failSet {
		return errCacheDown
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	if c.entries == nil {
		c.entries = map[string]entry{}
	}

	c.entries[key] = entry{value: value, expiresAt: expiresAt}

	return nil
}

func (c *fakeCache) stored(key string) (entry, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	e, ok := c.entries[key]

	return e, ok
}

type provider struct {
	*httptest.Server

	requests atomic.Int32
}

func newServer(t *testing.T, status int, body []byte) *provider {
	t.Helper()

	p := &provider{}
	p.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p.requests.Add(1)

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
	t.Cleanup(p.Close)

	return p
}

func readFixture(t *testing.T) []byte {
	t.Helper()

	fixture, err := os.ReadFile(_fixture)
	require.NoError(t, err)

	return fixture
}

func successBody(last, next time.Time, rates string) []byte {
	return fmt.Appendf(nil,
		`{"result":"success","base_code":"OMR","time_last_update_unix":%d,"time_next_update_unix":%d,"rates":{%s}}`,
		last.Unix(), next.Unix(), rates)
}

func freshBody(next time.Time) []byte {
	return successBody(time.Date(2026, 11, 8, 0, 2, 32, 0, time.UTC), next, `"CNY":`+_freshRate)
}

func errorBody(errorType string, next time.Time) []byte {
	return fmt.Appendf(nil, `{"result":"error","error-type":"%s","time_next_update_unix":%d}`, errorType, next.Unix())
}

func TestExchangeWebAPI_Rate(t *testing.T) {
	t.Parallel()

	fixture, err := os.ReadFile(_fixture)
	require.NoError(t, err)

	t.Run("reads the rate and its date from the fixture", func(t *testing.T) {
		t.Parallel()

		srv := newServer(t, http.StatusOK, fixture)

		rate, date, err := webapi.New(srv.URL, &fakeCache{}).Rate(t.Context(), "OMR", "CNY")

		require.NoError(t, err)
		assert.True(t, rate.Equal(decimal.RequireFromString("17.406483")), rate.String())
		assert.Equal(t, time.Date(2026, 10, 19, 0, 0, 0, 0, time.UTC), date)
	})

	t.Run("a status other than 200 is the provider being unavailable", func(t *testing.T) {
		t.Parallel()

		for _, status := range []int{
			http.StatusInternalServerError, http.StatusServiceUnavailable, http.StatusNotFound, http.StatusTooManyRequests,
		} {
			srv := newServer(t, status, nil)

			_, _, err := webapi.New(srv.URL, &fakeCache{}).Rate(t.Context(), "OMR", "CNY")

			require.ErrorIs(t, err, entity.ErrRatesUnavailable, status)
			assert.NotErrorIs(t, err, entity.ErrUnsupportedCurrency, status)
		}
	})

	t.Run("an unreachable provider is unavailable", func(t *testing.T) {
		t.Parallel()

		srv := newServer(t, http.StatusOK, fixture)
		srv.Close()

		_, _, err := webapi.New(srv.URL, &fakeCache{}).Rate(t.Context(), "OMR", "CNY")

		require.ErrorIs(t, err, entity.ErrRatesUnavailable)
	})

	t.Run("a canceled context stops the call, and is not the provider's failure", func(t *testing.T) {
		t.Parallel()

		srv := newServer(t, http.StatusOK, fixture)

		ctx, cancel := context.WithCancel(t.Context())
		cancel()

		_, _, err := webapi.New(srv.URL, &fakeCache{}).Rate(ctx, "OMR", "CNY")

		require.ErrorIs(t, err, context.Canceled)
		assert.NotErrorIs(t, err, entity.ErrRatesUnavailable)
	})
}

func TestExchangeWebAPI_RateErrorBodies(t *testing.T) {
	t.Parallel()

	// open.er-api.com reports errors with HTTP 200, so the client reads the body.
	tests := []struct {
		name string
		body string
		want *entity.Error
	}{
		{
			"unsupported-code means an unsupported currency",
			`{"result":"error","error-type":"unsupported-code"}`,
			entity.ErrUnsupportedCurrency,
		},
		{
			"a missing target rate means an unsupported currency",
			`{"result":"success","base_code":"OMR","time_last_update_unix":1792368152,"rates":{}}`,
			entity.ErrUnsupportedCurrency,
		},
		{
			"any other error is the provider being unavailable",
			`{"result":"error","error-type":"rate-limited"}`,
			entity.ErrRatesUnavailable,
		},
		{
			"a body that does not decode is the provider being unavailable",
			`{"result":"success","base_code":"OMR","rates":{"CNY":`,
			entity.ErrRatesUnavailable,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			srv := newServer(t, http.StatusOK, []byte(tt.body))

			_, _, err := webapi.New(srv.URL, &fakeCache{}).Rate(t.Context(), "OMR", "CNY")

			require.ErrorIs(t, err, tt.want)
			assert.Equal(t, entity.KindOf(tt.want), entity.KindOf(err))
		})
	}
}

func TestExchangeWebAPI_RateMissStores(t *testing.T) {
	t.Parallel()

	next := time.Now().Add(time.Hour)
	body := freshBody(next)
	srv := newServer(t, http.StatusOK, body)
	cache := &fakeCache{}
	api := webapi.New(srv.URL, cache)

	rate, date, err := api.Rate(t.Context(), "OMR", "CNY")

	require.NoError(t, err)
	assert.True(t, rate.Equal(decimal.RequireFromString(_freshRate)), rate.String())
	assert.Equal(t, time.Date(2026, 11, 8, 0, 0, 0, 0, time.UTC), date)
	assert.EqualValues(t, 1, srv.requests.Load())

	stored, ok := cache.stored(_cacheKey)
	require.True(t, ok)
	assert.Equal(t, body, stored.value)
	assert.True(t, stored.expiresAt.Equal(time.Unix(next.Unix(), 0)), stored.expiresAt)

	again, _, err := api.Rate(t.Context(), "OMR", "CNY")

	require.NoError(t, err)
	assert.True(t, again.Equal(rate), again.String())
	assert.EqualValues(t, 1, srv.requests.Load(), "the second conversion reads the cache")
}

func TestExchangeWebAPI_RateHits(t *testing.T) {
	t.Parallel()

	fixture := readFixture(t)
	cached := successBody(time.Date(2026, 10, 25, 0, 2, 32, 0, time.UTC), time.Now().Add(time.Hour), `"CNY":18.1`)

	tests := []struct {
		name string
		to   string
		rate string
		date time.Time
		err  error
	}{
		{"a hit returns the cached rate and date", "CNY", "18.1", time.Date(2026, 10, 25, 0, 0, 0, 0, time.UTC), nil},
		{"a target missing from the cached rates is an unsupported currency", "EUR", "0", time.Time{}, entity.ErrUnsupportedCurrency},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			srv := newServer(t, http.StatusOK, fixture)
			cache := &fakeCache{entries: map[string]entry{_cacheKey: {value: cached}}}

			rate, date, err := webapi.New(srv.URL, cache).Rate(t.Context(), "OMR", tt.to)

			require.ErrorIs(t, err, tt.err)
			assert.True(t, rate.Equal(decimal.RequireFromString(tt.rate)), rate.String())
			assert.Equal(t, tt.date, date)
			assert.Zero(t, srv.requests.Load(), "a hit makes no request")
		})
	}
}

func TestExchangeWebAPI_RateNeverStores(t *testing.T) {
	t.Parallel()

	fixture := readFixture(t)
	next := time.Now().Add(time.Hour)

	tests := []struct {
		name   string
		status int
		body   []byte
		err    error
	}{
		{"the recorded fixture, whose next update has passed", http.StatusOK, fixture, nil},
		{"an unsupported-code body", http.StatusOK, errorBody("unsupported-code", next), entity.ErrUnsupportedCurrency},
		{"any other error body", http.StatusOK, errorBody("rate-limited", next), entity.ErrRatesUnavailable},
		{"a status other than 200", http.StatusInternalServerError, freshBody(next), entity.ErrRatesUnavailable},
		{"a body that does not decode", http.StatusOK, []byte(`{"result":"success","rates":{"CNY":`), entity.ErrRatesUnavailable},
		{
			"a body over 1 MiB",
			http.StatusOK,
			successBody(next, next, `"CNY":`+_freshRate+strings.Repeat(" ", _maxBody)),
			entity.ErrRatesUnavailable,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			srv := newServer(t, tt.status, tt.body)
			cache := &fakeCache{}

			_, _, err := webapi.New(srv.URL, cache).Rate(t.Context(), "OMR", "CNY")

			require.ErrorIs(t, err, tt.err)
			assert.EqualValues(t, 1, srv.requests.Load())

			_, ok := cache.stored(_cacheKey)
			assert.False(t, ok, "nothing is stored")
		})
	}
}

func TestExchangeWebAPI_RateFailingCache(t *testing.T) {
	t.Parallel()

	body := freshBody(time.Now().Add(time.Hour))

	tests := []struct {
		name   string
		cache  *fakeCache
		stored bool
	}{
		{"a Get that fails is a miss", &fakeCache{failGet: true}, true},
		{"a Get that hangs is a miss", &fakeCache{hangGet: true}, true},
		{"a Set that fails still returns the rate", &fakeCache{failSet: true}, false},
		{
			"a cached body that does not decode is a miss, and the fresh body replaces it",
			&fakeCache{entries: map[string]entry{_cacheKey: {value: []byte(`{"result":`)}}},
			true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			srv := newServer(t, http.StatusOK, body)

			rate, _, err := webapi.New(srv.URL, tt.cache).Rate(t.Context(), "OMR", "CNY")

			require.NoError(t, err)
			assert.True(t, rate.Equal(decimal.RequireFromString(_freshRate)), rate.String())
			assert.EqualValues(t, 1, srv.requests.Load())

			stored, ok := tt.cache.stored(_cacheKey)
			require.Equal(t, tt.stored, ok)

			if tt.stored {
				assert.Equal(t, body, stored.value)
			}
		})
	}
}

func TestExchangeWebAPI_RateTimeout(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		<-r.Context().Done()
	}))
	t.Cleanup(srv.Close)

	api, ok := webapi.New(srv.URL, &fakeCache{}).(*webapi.ExchangeWebAPI)
	require.True(t, ok)
	api.SetTimeout(_shortTimeout)

	start := time.Now()
	_, _, err := api.Rate(t.Context(), "OMR", "CNY")

	require.ErrorIs(t, err, entity.ErrRatesUnavailable)
	assert.Less(t, time.Since(start), webapi.Timeout)
}
