package exchange

import (
	"github.com/m7medvision/go-clean-template/internal/repo"
	"github.com/m7medvision/go-clean-template/internal/usecase/exchange/command"
	"github.com/m7medvision/go-clean-template/internal/usecase/exchange/query"
	"github.com/m7medvision/go-clean-template/pkg/cqrs"
)

type Deps struct {
	Store     repo.EventStore
	Rates     repo.ExchangeWebAPI
	Exchanges repo.ExchangeReadModel
}

func Register(cmds *cqrs.Commands, qs *cqrs.Queries, d Deps) {
	cqrs.HandleCommand(cmds, command.ConvertCurrencyHandler{Store: d.Store, Rates: d.Rates})

	cqrs.HandleQuery(qs, query.GetExchangeHistoryHandler{Exchanges: d.Exchanges})
	cqrs.HandleQuery(qs, query.GetConversionHandler{Exchanges: d.Exchanges})
}
