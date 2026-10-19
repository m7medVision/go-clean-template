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

const _timeout = 5 * time.Second

var errUnexpectedStatus = errors.New("unexpected status")

// ExchangeWebAPI reads European Central Bank reference rates from the Frankfurter API
// (https://frankfurter.dev): keyless, documented, and self-hostable.
type ExchangeWebAPI struct {
	baseURL string
	client  *http.Client
}

// latestResponse is the part of GET /v1/latest the client reads. Rates decode straight into
// decimal.Decimal, so a rate never passes through float64.
type latestResponse struct {
	Date  string                     `json:"date"`
	Rates map[string]decimal.Decimal `json:"rates"`
}

// New returns a Frankfurter client for baseURL (e.g. https://api.frankfurter.dev),
// instrumented with OpenTelemetry tracing spans.
func New(baseURL string) repo.ExchangeWebAPI {
	return newTraced(&ExchangeWebAPI{
		baseURL: strings.TrimSuffix(baseURL, "/"),
		client:  &http.Client{Timeout: _timeout, Transport: otelhttp.NewTransport(http.DefaultTransport)},
	})
}

// Rate -.
func (w *ExchangeWebAPI) Rate(ctx context.Context, from, to string) (decimal.Decimal, time.Time, error) {
	query := url.Values{"base": {from}, "symbols": {to}}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, w.baseURL+"/v1/latest?"+query.Encode(), http.NoBody)
	if err != nil {
		return decimal.Decimal{}, time.Time{}, fmt.Errorf("ExchangeWebAPI - Rate - http.NewRequestWithContext: %w", err)
	}

	resp, err := w.client.Do(req)
	if err != nil {
		return decimal.Decimal{}, time.Time{}, fmt.Errorf("ExchangeWebAPI - Rate - w.client.Do: %w", err)
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusOK:
	case http.StatusNotFound:
		return decimal.Decimal{}, time.Time{}, entity.ErrUnsupportedCurrency
	default:
		return decimal.Decimal{}, time.Time{}, fmt.Errorf("ExchangeWebAPI - Rate - %w: %d", errUnexpectedStatus, resp.StatusCode)
	}

	var body latestResponse
	if err = json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return decimal.Decimal{}, time.Time{}, fmt.Errorf("ExchangeWebAPI - Rate - json.Decode: %w", err)
	}

	rate, ok := body.Rates[to]
	if !ok {
		return decimal.Decimal{}, time.Time{}, entity.ErrUnsupportedCurrency
	}

	date, err := time.Parse(time.DateOnly, body.Date)
	if err != nil {
		return decimal.Decimal{}, time.Time{}, fmt.Errorf("ExchangeWebAPI - Rate - time.Parse: %w", err)
	}

	return rate, date, nil
}
