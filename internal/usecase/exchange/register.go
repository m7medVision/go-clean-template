package exchange

import (
	"github.com/m7medvision/go-clean-template/internal/repo"
	"github.com/m7medvision/go-clean-template/internal/usecase/exchange/command"
	"github.com/m7medvision/go-clean-template/internal/usecase/exchange/query"
	"github.com/m7medvision/go-clean-template/pkg/cqrs"
)

// Deps is what the exchange handlers are built from.
type Deps struct {
	Store   repo.EventStore
	Rates   repo.ExchangeWebAPI
	History repo.ExchangeReadModel
}

// Register registers every exchange command and query handler, one line each.
func Register(cmds *cqrs.Commands, qs *cqrs.Queries, d Deps) {
	cqrs.HandleCommand(cmds, command.ConvertCurrencyHandler{Store: d.Store, Rates: d.Rates})

	cqrs.HandleQuery(qs, query.GetExchangeHistoryHandler{History: d.History})
	cqrs.HandleQuery(qs, query.GetConversionHandler{History: d.History})
}
