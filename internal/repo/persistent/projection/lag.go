package projection

import (
	"context"
	"fmt"
	"time"

	"github.com/m7medvision/go-clean-template/internal/repo/persistent/queries"
	"github.com/m7medvision/go-clean-template/pkg/postgres"
	"github.com/prometheus/client_golang/prometheus"
)

const _lagTimeout = 2 * time.Second

// LagCollector reports projection_lag_events{projector}: how many events each projector and
// event handler is behind the last one stored. It reads the checkpoints on every scrape, so the app,
// which serves /metrics, reports the lag of subscribers that run in the worker.
type LagCollector struct {
	pg   *postgres.Postgres
	desc *prometheus.Desc
}

func NewLagCollector(pg *postgres.Postgres) *LagCollector {
	return &LagCollector{
		pg: pg,
		desc: prometheus.NewDesc("projection_lag_events",
			"Events stored after each projector's or event handler's checkpoint.", []string{"projector"}, nil),
	}
}

func (c *LagCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- c.desc
}

func (c *LagCollector) Collect(ch chan<- prometheus.Metric) {
	ctx, cancel := context.WithTimeout(context.Background(), _lagTimeout)
	defer cancel()

	rows, err := queries.New(c.pg.Executor(ctx)).CheckpointLag(ctx)
	if err != nil {
		ch <- prometheus.NewInvalidMetric(c.desc, fmt.Errorf("projection - LagCollector - q.CheckpointLag: %w", err))

		return
	}

	for _, row := range rows {
		ch <- prometheus.MustNewConstMetric(c.desc, prometheus.GaugeValue, float64(row.Lag), row.Name)
	}
}
