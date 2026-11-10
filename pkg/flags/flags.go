// Package flags evaluates feature flags through OpenFeature. With a file it reads the flags from it
// through GO Feature Flag; without one every flag answers the default its caller passes.
package flags

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	otelhooks "github.com/open-feature/go-sdk-contrib/hooks/open-telemetry/pkg"
	gofeatureflaginprocess "github.com/open-feature/go-sdk-contrib/providers/go-feature-flag-in-process/pkg"
	"github.com/open-feature/go-sdk/openfeature"
	"github.com/open-feature/go-sdk/openfeature/isolated"
	ffclient "github.com/thomaspoignant/go-feature-flag"
	"github.com/thomaspoignant/go-feature-flag/retriever/fileretriever"
)

const _pollInterval = 10 * time.Second

type Client struct {
	api    *openfeature.EvaluationAPI
	client *openfeature.Client
}

// New reads the flags from file, polled every few seconds, or answers every flag's default when
// file is empty.
func New(ctx context.Context, file string, l *slog.Logger) (*Client, error) {
	if file == "" {
		return NewWithProvider(ctx, openfeature.NoopProvider{}, l)
	}

	p, err := gofeatureflaginprocess.NewProviderWithContext(ctx, gofeatureflaginprocess.ProviderOptions{
		GOFeatureFlagConfig: &ffclient.Config{
			PollingInterval: _pollInterval,
			Context:         ctx,
			LeveledLogger:   l,
			Retriever:       &fileretriever.Retriever{Path: file},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("flags.New: %w", err)
	}

	return NewWithProvider(ctx, p, l)
}

// NewWithProvider evaluates through p, with the trace and log hooks every client carries.
func NewWithProvider(ctx context.Context, p openfeature.FeatureProvider, l *slog.Logger) (*Client, error) {
	api := isolated.NewAPI()
	api.AddHooks(otelhooks.NewTracesHook(), logHook{l: l})

	if err := api.SetProviderAndWait(ctx, p); err != nil {
		return nil, fmt.Errorf("flags.NewWithProvider: %w", err)
	}

	return &Client{api: api, client: api.NewClient()}, nil
}

// Bool is the flag key for userID, or def when the flag cannot be evaluated.
func (c *Client) Bool(ctx context.Context, key string, def bool, userID string) bool {
	v, _ := c.client.BooleanValue(ctx, key, def, openfeature.NewEvaluationContext(userID, nil)) //nolint:errcheck // logHook logs it and v is def

	return v
}

func (c *Client) Close(ctx context.Context) error {
	if err := c.api.Shutdown(ctx); err != nil {
		return fmt.Errorf("flags.Close: %w", err)
	}

	return nil
}

type logHook struct {
	openfeature.UnimplementedHook

	l *slog.Logger
}

//nolint:gocritic // the Hook interface takes HookContext by value
func (h logHook) Error(ctx context.Context, hc openfeature.HookContext, err error, _ openfeature.HookHints) {
	h.l.WarnContext(ctx, "flag evaluation failed, default used", "flag", hc.FlagKey(), "error", err)
}
