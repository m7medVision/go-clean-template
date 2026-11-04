package exchange

import (
	"context"
	"net/http"

	"github.com/danielgtaylor/huma/v2"
	"github.com/m7medvision/go-clean-template/internal/controller/restapi/middleware"
	"github.com/m7medvision/go-clean-template/internal/controller/restapi/v1/apidoc"
	"github.com/m7medvision/go-clean-template/internal/entity"
	"github.com/m7medvision/go-clean-template/internal/usecase/exchange/command"
	"github.com/m7medvision/go-clean-template/internal/usecase/exchange/query"
	"github.com/m7medvision/go-clean-template/pkg/cqrs"
	"github.com/shopspring/decimal"
)

type convertInput struct {
	Body Convert
}

type convertOutput struct {
	Body Conversion
}

type exchangeHistoryInput struct {
	Limit  int `query:"limit" default:"10" minimum:"1" maximum:"100"`
	Offset int `query:"offset" default:"0" minimum:"0"`
}

type exchangeHistoryOutput struct {
	Body ExchangeHistory
}

func (r *controller) convert(api huma.API) {
	huma.Register(api, huma.Operation{
		OperationID: "convert-currency",
		Summary:     "Convert currency",
		Description: "Convert an amount at the latest daily rate and save it to the history",
		Method:      http.MethodPost,
		Path:        "/v1/exchange/convert",
		Tags:        []string{_exchangeTag},
		Security:    []map[string][]string{{apidoc.BearerAuthScheme: {}}},
		Errors:      []int{http.StatusBadRequest, http.StatusUnauthorized, http.StatusInternalServerError},
	}, func(ctx context.Context, input *convertInput) (*convertOutput, error) {
		userID, ok := middleware.UserID(ctx)
		if !ok {
			return nil, huma.Error401Unauthorized("unauthorized")
		}

		amount, err := decimal.NewFromString(input.Body.Amount)
		if err != nil {
			return nil, huma.Error422UnprocessableEntity("invalid amount")
		}

		written, err := cqrs.SendCommand[cqrs.Written](ctx, r.cmds, command.ConvertCurrency{
			UserID: userID, From: input.Body.From, To: input.Body.To, Amount: amount,
		})
		if err != nil {
			return nil, r.problem(err, "convert")
		}

		conversion, err := cqrs.SendQuery[entity.Conversion](ctx, r.qs,
			query.GetConversion{UserID: userID, ConversionID: written.ID})
		if err != nil {
			return nil, r.problem(err, "convert")
		}

		return &convertOutput{Body: NewConversion(&conversion)}, nil
	})
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
		Errors:      []int{http.StatusUnauthorized, http.StatusInternalServerError},
	}, func(ctx context.Context, input *exchangeHistoryInput) (*exchangeHistoryOutput, error) {
		userID, ok := middleware.UserID(ctx)
		if !ok {
			return nil, huma.Error401Unauthorized("unauthorized")
		}

		history, err := cqrs.SendQuery[entity.ExchangeHistory](ctx, r.qs,
			query.GetExchangeHistory{UserID: userID, Limit: input.Limit, Offset: input.Offset})
		if err != nil {
			return nil, r.problem(err, "exchangeHistory")
		}

		return &exchangeHistoryOutput{Body: NewExchangeHistory(history)}, nil
	})
}
