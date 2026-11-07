package exchange

import (
	"context"
	"net/http"

	"github.com/danielgtaylor/huma/v2"
	"github.com/m7medvision/go-clean-template/internal/controller/restapi/middleware"
	"github.com/m7medvision/go-clean-template/internal/controller/restapi/v1/apidoc"
	"github.com/m7medvision/go-clean-template/internal/controller/restapi/v1/apierr"
	"github.com/m7medvision/go-clean-template/internal/usecase/exchange/query"
	"github.com/m7medvision/go-clean-template/pkg/cqrs"
)

type exchangeHistoryInput struct {
	Limit  int `query:"limit" default:"10" minimum:"1" maximum:"100"`
	Offset int `query:"offset" default:"0" minimum:"0"`
}

type exchangeHistoryOutput struct {
	Body ExchangeHistory
}

func (r *controller) exchangeHistory(api huma.API) {
	huma.Register(api, huma.Operation{
		OperationID: "exchange-history",
		Summary:     "Show conversion history",
		Description: "Show one page of the current user's currency conversions, newest first",
		Method:      http.MethodGet,
		Path:        "/v1/exchange/history",
		Tags:        []string{_exchangeTag},
		Security:    []map[string][]string{{apidoc.BearerAuthScheme: {}}},
		Errors:      apierr.QueryErrors(http.StatusUnauthorized),
	}, func(ctx context.Context, input *exchangeHistoryInput) (*exchangeHistoryOutput, error) {
		userID, ok := middleware.UserID(ctx)
		if !ok {
			return nil, huma.Error401Unauthorized("unauthorized")
		}

		history, err := cqrs.SendQuery(ctx, r.qs,
			query.GetExchangeHistory{UserID: userID, Limit: input.Limit, Offset: input.Offset})
		if err != nil {
			return nil, apierr.Problem(ctx, r.l, err, "exchangeHistory")
		}

		return &exchangeHistoryOutput{Body: NewExchangeHistory(history)}, nil
	})
}
