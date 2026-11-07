package exchange

import (
	"context"
	"net/http"

	"github.com/danielgtaylor/huma/v2"
	"github.com/m7medvision/go-clean-template/internal/controller/authn"
	"github.com/m7medvision/go-clean-template/internal/controller/restapi/v1/apidoc"
	"github.com/m7medvision/go-clean-template/internal/controller/restapi/v1/apierr"
	"github.com/m7medvision/go-clean-template/internal/usecase/exchange/command"
	"github.com/m7medvision/go-clean-template/internal/usecase/exchange/query"
	"github.com/m7medvision/go-clean-template/pkg/cqrs"
)

type convertInput struct {
	Body Convert
}

type convertOutput struct {
	Body Conversion
}

func (r *controller) convert(api huma.API) {
	huma.Register(api, huma.Operation{
		OperationID: "convert-currency",
		Summary:     "Convert currency",
		Description: "Convert an amount at the latest daily rate and save it to the history",
		Method:      http.MethodPost,
		Path:        "/v1/exchange/convert",
		Tags:        []string{_tag},
		Security:    []map[string][]string{{apidoc.BearerAuthScheme: {}}},
		Errors:      apierr.CommandErrors(http.StatusBadRequest, http.StatusUnauthorized),
	}, func(ctx context.Context, input *convertInput) (*convertOutput, error) {
		userID := authn.MustCaller(ctx).UserID

		written, err := cqrs.SendCommand(ctx, r.cmds, command.ConvertCurrency{
			UserID: userID, From: input.Body.From, To: input.Body.To, Amount: input.Body.Amount,
		})
		if err != nil {
			return nil, apierr.Problem(ctx, r.l, err, "convert")
		}

		conversion, err := cqrs.SendQuery(ctx, r.qs,
			query.GetConversion{UserID: userID, ConversionID: written.ID})
		if err != nil {
			return nil, apierr.Problem(ctx, r.l, err, "convert")
		}

		return &convertOutput{Body: NewConversion(&conversion)}, nil
	})
}
