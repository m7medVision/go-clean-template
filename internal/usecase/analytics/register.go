// Package analytics delivers analytics events: its one command, DeliverAnalytics, lives in command/.
package analytics

import (
	"github.com/m7medvision/go-clean-template/internal/repo"
	"github.com/m7medvision/go-clean-template/internal/usecase/analytics/command"
	"github.com/m7medvision/go-clean-template/pkg/cqrs"
)

type Deps struct {
	Sinks map[string]repo.AnalyticsSink
}

// Register registers the analytics command handler; analytics answers no queries.
func Register(cmds *cqrs.Commands, _ *cqrs.Queries, d Deps) {
	cqrs.HandleCommand(cmds, command.DeliverAnalyticsHandler{Sinks: d.Sinks})
}
