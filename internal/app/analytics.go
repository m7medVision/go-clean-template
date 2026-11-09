package app

import (
	"maps"
	"slices"

	"github.com/m7medvision/go-clean-template/config"
	workerv1 "github.com/m7medvision/go-clean-template/internal/controller/worker/v1"
	"github.com/m7medvision/go-clean-template/internal/repo"
	analyticsrepo "github.com/m7medvision/go-clean-template/internal/repo/analytics"
	"github.com/m7medvision/go-clean-template/internal/repo/analytics/posthog"
	"github.com/m7medvision/go-clean-template/internal/repo/notify"
	analyticsuc "github.com/m7medvision/go-clean-template/internal/usecase/analytics"
	"github.com/m7medvision/go-clean-template/pkg/cqrs"
)

// analyticsModule is analytics delivery, which only the worker runs: one topic per configured sink,
// and the DeliverAnalytics handler its messages send to. With no sink it registers nothing.
func analyticsModule() module {
	return module{
		queues: func(d *deps, b cqrs.Buses, r queueRouter) error {
			if len(d.sinks) == 0 {
				return nil
			}

			analyticsuc.Register(b.Commands, b.Queries, analyticsuc.Deps{Sinks: d.sinks})
			workerv1.NewAnalyticsRoutes(r, b.Commands, sinkNames(d.sinks), d.l)

			return nil
		},
	}
}

// analyticsSinks returns a sink for every vendor whose settings are present. A new vendor adds its
// sink here.
func analyticsSinks(cfg config.Analytics) map[string]repo.AnalyticsSink {
	sinks := map[string]repo.AnalyticsSink{}

	if cfg.PostHog.APIKey != "" {
		s := posthog.New(cfg.PostHog.APIKey, cfg.PostHog.Host)
		sinks[s.Name()] = s
	}

	return sinks
}

// newAnalytics is the publisher the capture-* event handlers send through; nil with no sink, so
// they are not installed.
func newAnalytics(pub notify.Publisher, sinks map[string]repo.AnalyticsSink) repo.Analytics {
	if len(sinks) == 0 {
		return nil
	}

	return analyticsrepo.New(pub, sinkNames(sinks))
}

func sinkNames(sinks map[string]repo.AnalyticsSink) []string {
	return slices.Sorted(maps.Keys(sinks))
}

// capture is a module's analytics event handler, or none when analytics is off.
func capture(d *deps, handler func(a repo.Analytics) repo.Subscriber) []repo.Subscriber {
	if d.analytics == nil {
		return nil
	}

	return []repo.Subscriber{handler(d.analytics)}
}
