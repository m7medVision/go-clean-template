// Package webapi implements the repositories backed by outbound HTTP APIs.
package webapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/m7medvision/go-clean-template/internal/entity"
	"github.com/m7medvision/go-clean-template/internal/repo"
	"github.com/shopspring/decimal"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
)

// Timeout bounds one Rate: the cache read, the request and the cache write, all of them together.
// The app's HTTP write deadline leaves room for it, since a conversion waits for its rate before it
// is saved.
const Timeout = 5 * time.Second

const (
	_resultSuccess   = "success"
	_unsupportedCode = "unsupported-code"
	_cacheKeyPrefix  = "exchange:latest:"
	_cacheTimeout    = 500 * time.Millisecond
	_maxBody         = 1 << 20
)

var errTimedOut = errors.New("webapi: the provider did not answer in time")

type Cache interface {
	Get(ctx context.Context, key string) ([]byte, bool, error)
	Set(ctx context.Context, key string, value []byte, expiresAt time.Time) error
}

// ExchangeWebAPI reads daily rates from the open access ExchangeRate-API endpoint
// (https://www.exchangerate-api.com/docs/free): keyless, about 160 currencies. It caches a base's
// rates until the provider's next update. Its terms ask every page that shows the rates to link
// back to it. A currency the provider does not know is entity.ErrUnsupportedCurrency. Any other
// failure of the provider is entity.ErrRatesUnavailable: one that cannot be reached or times out,
// answers a status other than 200, reports another error, or sends a body that does not decode.
type ExchangeWebAPI struct {
	baseURL string
	client  *http.Client
	cache   Cache
	timeout time.Duration
}

// latestResponse is the part of GET /v6/latest/{base} the client reads. Rates decode straight
// into decimal.Decimal, so a rate never passes through float64.
type latestResponse struct {
	Result     string                     `json:"result"`
	ErrorType  string                     `json:"error-type"`
	LastUpdate int64                      `json:"time_last_update_unix"`
	NextUpdate int64                      `json:"time_next_update_unix"`
	Rates      map[string]decimal.Decimal `json:"rates"`
}

// New returns an ExchangeRate-API client for baseURL (e.g. https://open.er-api.com). Its transport
// opens an OpenTelemetry client span per request.
func New(baseURL string, cache Cache) repo.ExchangeWebAPI {
	return &ExchangeWebAPI{
		baseURL: strings.TrimSuffix(baseURL, "/"),
		client:  &http.Client{Timeout: Timeout, Transport: otelhttp.NewTransport(http.DefaultTransport)},
		cache:   cache,
		timeout: Timeout,
	}
}

func (w *ExchangeWebAPI) Rate(ctx context.Context, from, to string) (decimal.Decimal, time.Time, error) {
	ctx, cancel := context.WithTimeoutCause(ctx, w.timeout, errTimedOut)
	defer cancel()

	body, err := w.latest(ctx, from)
	if err != nil {
		return decimal.Decimal{}, time.Time{}, err
	}

	// The provider reports errors with HTTP 200 and result "error". Only unsupported-code is about
	// the caller's input; any other is the provider failing.
	if body.Result != _resultSuccess {
		if body.ErrorType == _unsupportedCode {
			return decimal.Decimal{}, time.Time{}, entity.ErrUnsupportedCurrency
		}

		return decimal.Decimal{}, time.Time{},
			fmt.Errorf("ExchangeWebAPI - Rate - %w: error-type %q", entity.ErrRatesUnavailable, body.ErrorType)
	}

	rate, ok := body.Rates[to]
	if !ok {
		return decimal.Decimal{}, time.Time{}, entity.ErrUnsupportedCurrency
	}

	updated := time.Unix(body.LastUpdate, 0).UTC()
	date := time.Date(updated.Year(), updated.Month(), updated.Day(), 0, 0, 0, 0, time.UTC)

	return rate, date, nil
}

func (w *ExchangeWebAPI) latest(ctx context.Context, base string) (latestResponse, error) {
	key := _cacheKeyPrefix + base

	if body, ok := w.cached(ctx, key); ok {
		return body, nil
	}

	raw, body, err := w.fetch(ctx, base)
	if err != nil {
		return latestResponse{}, err
	}

	if next := time.Unix(body.NextUpdate, 0); body.Result == _resultSuccess && next.After(time.Now()) {
		w.store(ctx, key, raw, next)
	}

	return body, nil
}

func (w *ExchangeWebAPI) cached(ctx context.Context, key string) (latestResponse, bool) {
	ctx, cancel := context.WithTimeout(ctx, _cacheTimeout)
	defer cancel()

	raw, ok, err := w.cache.Get(ctx, key)
	if err != nil || !ok {
		return latestResponse{}, false
	}

	var body latestResponse
	if json.Unmarshal(raw, &body) != nil {
		return latestResponse{}, false
	}

	return body, true
}

func (w *ExchangeWebAPI) fetch(ctx context.Context, base string) ([]byte, latestResponse, error) {
	endpoint := w.baseURL + "/v6/latest/" + url.PathEscape(base)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, http.NoBody)
	if err != nil {
		return nil, latestResponse{}, fmt.Errorf("ExchangeWebAPI - fetch - http.NewRequestWithContext: %w", err)
	}

	resp, err := w.client.Do(req)
	if err != nil {
		return nil, latestResponse{}, unavailable(ctx, "ExchangeWebAPI - fetch - w.client.Do", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, latestResponse{}, fmt.Errorf("ExchangeWebAPI - fetch - %w: status %d", entity.ErrRatesUnavailable, resp.StatusCode)
	}

	raw, err := io.ReadAll(io.LimitReader(resp.Body, _maxBody))
	if err != nil {
		return nil, latestResponse{}, unavailable(ctx, "ExchangeWebAPI - fetch - io.ReadAll", err)
	}

	var body latestResponse
	if err = json.Unmarshal(raw, &body); err != nil {
		return nil, latestResponse{}, unavailable(ctx, "ExchangeWebAPI - fetch - json.Unmarshal", err)
	}

	return raw, body, nil
}

func (w *ExchangeWebAPI) store(ctx context.Context, key string, raw []byte, next time.Time) {
	ctx, cancel := context.WithTimeout(ctx, _cacheTimeout)
	defer cancel()

	_ = w.cache.Set(ctx, key, raw, next) //nolint:errcheck // a cache that fails only costs the next conversion a request to the provider
}

// unavailable is err, from op's exchange with the provider, as entity.ErrRatesUnavailable, unless
// the caller went away first: a canceled request is not the provider's failure.
func unavailable(ctx context.Context, op string, err error) error {
	if ctx.Err() != nil && !errors.Is(context.Cause(ctx), errTimedOut) {
		return fmt.Errorf("%s: %w", op, err)
	}

	return fmt.Errorf("%s: %w: %w", op, entity.ErrRatesUnavailable, err)
}
