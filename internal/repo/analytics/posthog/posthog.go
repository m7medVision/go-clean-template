// Package posthog is the PostHog analytics sink: the only code that knows PostHog.
package posthog

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/m7medvision/go-clean-template/internal/entity/analytics"
	"github.com/m7medvision/go-clean-template/internal/repo"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
)

const (
	_name        = "posthog"
	_capturePath = "/i/v0/e/"
	_timeout     = 10 * time.Second
	_maxBody     = 1 << 16
)

var errRetry = errors.New("posthog: try again")

// Sink sends one event per request to PostHog's capture API and waits for its answer, so a failure
// reaches the queue. The event's ID becomes PostHog's uuid, which PostHog deduplicates on.
type Sink struct {
	apiKey  string
	baseURL string
	client  *http.Client
}

var _ repo.AnalyticsSink = (*Sink)(nil)

func New(apiKey, host string) *Sink {
	return &Sink{
		apiKey:  apiKey,
		baseURL: strings.TrimSuffix(host, "/"),
		client:  &http.Client{Timeout: _timeout, Transport: otelhttp.NewTransport(http.DefaultTransport)},
	}
}

func (*Sink) Name() string { return _name }

// capture is PostHog's capture request, which carries the project's public key in its body.
type capture struct {
	APIKey     string          `json:"api_key"`
	Event      string          `json:"event"`
	DistinctID string          `json:"distinct_id"`
	UUID       string          `json:"uuid"`
	Timestamp  time.Time       `json:"timestamp"`
	Properties json.RawMessage `json:"properties"`
}

func (s *Sink) Send(ctx context.Context, e *analytics.Event) error {
	props, err := json.Marshal(e.Properties)
	if err != nil {
		return fmt.Errorf("posthog - Send - json.Marshal properties: %w", err)
	}

	body, err := json.Marshal(capture{ //nolint:gosec // PostHog's write-only project key, sent in the body as its API requires
		APIKey:     s.apiKey,
		Event:      e.Properties.Name(),
		DistinctID: e.DistinctID,
		UUID:       UUID(e.ID),
		Timestamp:  e.At,
		Properties: props,
	})
	if err != nil {
		return fmt.Errorf("posthog - Send - json.Marshal: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.baseURL+_capturePath, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("posthog - Send - http.NewRequestWithContext: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := s.client.Do(req)
	if err != nil {
		return fmt.Errorf("posthog - Send - client.Do: %w", err)
	}
	defer resp.Body.Close()

	if _, err = io.Copy(io.Discard, io.LimitReader(resp.Body, _maxBody)); err != nil {
		return fmt.Errorf("posthog - Send - read body: %w", err)
	}

	return status(resp.StatusCode)
}

// UUID is the PostHog uuid for an event ID: the same ID always gives the same uuid.
func UUID(id string) string {
	return uuid.NewSHA1(uuid.NameSpaceOID, []byte(id)).String()
}

func status(code int) error {
	switch {
	case code >= http.StatusOK && code < http.StatusMultipleChoices:
		return nil
	case code == http.StatusTooManyRequests || code >= http.StatusInternalServerError:
		return fmt.Errorf("posthog - Send: %w: %d", errRetry, code)
	default:
		return fmt.Errorf("posthog - Send: %w: %d", analytics.ErrRejected, code)
	}
}
