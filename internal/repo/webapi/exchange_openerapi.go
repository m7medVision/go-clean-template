// Package webapi implements the repositories backed by outbound HTTP APIs.
package webapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/evrone/go-clean-template/internal/entity"
	"github.com/evrone/go-clean-template/internal/repo"
	"github.com/shopspring/decimal"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
)

const (
	_timeout = 5 * time.Second

	_resultSuccess   = "success"
	_unsupportedCode = "unsupported-code"
)

var (
	errUnexpectedStatus = errors.New("unexpected status")
	errProvider         = errors.New("rate provider error")
)

// ExchangeWebAPI reads daily rates from the open access ExchangeRate-API endpoint
// (https://www.exchangerate-api.com/docs/free): keyless, about 160 currencies. Its terms ask
// every page that shows the rates to link back to it.
type ExchangeWebAPI struct {
	baseURL string
	client  *http.Client
}

// latestResponse is the part of GET /v6/latest/{base} the client reads. Rates decode straight
// into decimal.Decimal, so a rate never passes through float64.
type latestResponse struct {
	Result     string                     `json:"result"`
	ErrorType  string                     `json:"error-type"`
	LastUpdate int64                      `json:"time_last_update_unix"`
	Rates      map[string]decimal.Decimal `json:"rates"`
}

// New returns an ExchangeRate-API client for baseURL (e.g. https://open.er-api.com),
// instrumented with OpenTelemetry tracing spans.
func New(baseURL string) repo.ExchangeWebAPI {
	return newTraced(&ExchangeWebAPI{
		baseURL: strings.TrimSuffix(baseURL, "/"),
		client:  &http.Client{Timeout: _timeout, Transport: otelhttp.NewTransport(http.DefaultTransport)},
	})
}

// Rate -.
func (w *ExchangeWebAPI) Rate(ctx context.Context, from, to string) (decimal.Decimal, time.Time, error) {
	body, err := w.latest(ctx, from)
	if err != nil {
		return decimal.Decimal{}, time.Time{}, err
	}

	// The provider reports errors with HTTP 200 and result "error".
	if body.Result != _resultSuccess {
		if body.ErrorType == _unsupportedCode {
			return decimal.Decimal{}, time.Time{}, entity.ErrUnsupportedCurrency
		}

		return decimal.Decimal{}, time.Time{}, fmt.Errorf("ExchangeWebAPI - Rate - %w: %s", errProvider, body.ErrorType)
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
	endpoint := w.baseURL + "/v6/latest/" + url.PathEscape(base)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, http.NoBody)
	if err != nil {
		return latestResponse{}, fmt.Errorf("ExchangeWebAPI - latest - http.NewRequestWithContext: %w", err)
	}

	resp, err := w.client.Do(req)
	if err != nil {
		return latestResponse{}, fmt.Errorf("ExchangeWebAPI - latest - w.client.Do: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return latestResponse{}, fmt.Errorf("ExchangeWebAPI - latest - %w: %d", errUnexpectedStatus, resp.StatusCode)
	}

	var body latestResponse
	if err = json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return latestResponse{}, fmt.Errorf("ExchangeWebAPI - latest - json.Decode: %w", err)
	}

	return body, nil
}
