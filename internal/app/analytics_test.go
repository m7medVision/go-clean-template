package app

import (
	"context"
	"testing"

	"github.com/m7medvision/go-clean-template/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type nopPublisher struct{}

func (nopPublisher) Publish(context.Context, string, string, []byte) error { return nil }

func TestWithNoAnalyticsVendorNothingIsInstalled(t *testing.T) {
	t.Parallel()

	d := testDeps(nil)
	d.sinks = analyticsSinks(config.Analytics{})
	d.analytics = newAnalytics(nopPublisher{}, d.sinks)

	r := &fakeRouter{}
	require.NoError(t, registerQueues(d, newTestBuses(), r))
	assert.NotContains(t, r.topics, "analytics-posthog")

	for _, h := range eventHandlers(d) {
		assert.NotContains(t, h.Name(), "capture-")
	}
}

func TestPostHogIsOnWhenItsKeyIsSet(t *testing.T) {
	t.Parallel()

	d := testDeps(nil)
	d.sinks = analyticsSinks(config.Analytics{PostHog: config.PostHog{APIKey: "key", Host: "http://localhost"}})
	d.analytics = newAnalytics(nopPublisher{}, d.sinks)

	r := &fakeRouter{}
	require.NoError(t, registerQueues(d, newTestBuses(), r))
	assert.Contains(t, r.topics, "analytics-posthog")

	var names []string
	for _, h := range eventHandlers(d) {
		names = append(names, h.Name())
	}

	assert.Subset(t, names, []string{"capture-user", "capture-task", "capture-exchange"})
}
