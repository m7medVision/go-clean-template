package v1

import (
	"context"
	"errors"
	"net/http"

	"github.com/danielgtaylor/huma/v2"
	"github.com/evrone/go-clean-template/internal/controller/restapi/middleware"
	"github.com/evrone/go-clean-template/internal/controller/restapi/v1/request"
	"github.com/evrone/go-clean-template/internal/controller/restapi/v1/response"
	"github.com/evrone/go-clean-template/internal/entity"
	"github.com/shopspring/decimal"
)

type convertInput struct {
	Body request.Convert
}

type convertOutput struct {
	Body response.Conversion
}

type exchangeHistoryInput struct{}

type exchangeHistoryOutput struct {
	Body response.ExchangeHistory
}

func (r *V1) convert(api huma.API) {
	huma.Register(api, huma.Operation{
		OperationID: "convert-currency",
		Summary:     "Convert currency",
		Description: "Convert an amount at the latest European Central Bank reference rate and save it to the history",
		Method:      http.MethodPost,
		Path:        "/v1/exchange/convert",
		Tags:        []string{_exchangeTag},
		Security:    []map[string][]string{{_bearerAuthScheme: {}}},
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

		conversion, err := r.e.Convert(ctx, userID, input.Body.From, input.Body.To, amount)
		if err != nil {
			if errors.Is(err, entity.ErrInvalidConversion) {
				return nil, huma.Error400BadRequest("invalid conversion")
			}

			if errors.Is(err, entity.ErrUnsupportedCurrency) {
				return nil, huma.Error400BadRequest("unsupported currency")
			}

			r.l.Error(err, "restapi - v1 - convert")

			return nil, huma.Error500InternalServerError("internal server error")
		}

		return &convertOutput{Body: response.NewConversion(&conversion)}, nil
	})
}

func (r *V1) exchangeHistory(api huma.API) {
	huma.Register(api, huma.Operation{
		OperationID: "exchange-history",
		Summary:     "Show conversion history",
		Description: "Show the current user's most recent currency conversions, newest first",
		Method:      http.MethodGet,
		Path:        "/v1/exchange/history",
		Tags:        []string{_exchangeTag},
		Security:    []map[string][]string{{_bearerAuthScheme: {}}},
		Errors:      []int{http.StatusUnauthorized, http.StatusInternalServerError},
	}, func(ctx context.Context, _ *exchangeHistoryInput) (*exchangeHistoryOutput, error) {
		userID, ok := middleware.UserID(ctx)
		if !ok {
			return nil, huma.Error401Unauthorized("unauthorized")
		}

		history, err := r.e.History(ctx, userID)
		if err != nil {
			r.l.Error(err, "restapi - v1 - exchangeHistory")

			return nil, huma.Error500InternalServerError("internal server error")
		}

		return &exchangeHistoryOutput{Body: response.NewExchangeHistory(history)}, nil
	})
}
