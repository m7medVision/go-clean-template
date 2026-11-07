package exchange

import (
	"context"

	v1 "github.com/m7medvision/go-clean-template/docs/proto/v1"
	"github.com/m7medvision/go-clean-template/internal/controller/authn"
	"github.com/m7medvision/go-clean-template/internal/controller/grpc/v1/rpcerr"
	"github.com/m7medvision/go-clean-template/internal/usecase/exchange/command"
	"github.com/m7medvision/go-clean-template/internal/usecase/exchange/query"
	"github.com/m7medvision/go-clean-template/pkg/cqrs"
)

func (c *Controller) Convert(ctx context.Context, req *v1.ConvertRequest) (*v1.ConversionResponse, error) {
	userID := authn.MustCaller(ctx).UserID

	written, err := cqrs.SendCommand(ctx, c.cmds, command.ConvertCurrency{
		UserID: userID, From: req.GetFrom(), To: req.GetTo(), Amount: req.GetAmount(),
	})
	if err != nil {
		return nil, rpcerr.Status(ctx, c.l, err, "Convert")
	}

	conversion, err := cqrs.SendQuery(ctx, c.qs,
		query.GetConversion{UserID: userID, ConversionID: written.ID})
	if err != nil {
		return nil, rpcerr.Status(ctx, c.l, err, "Convert")
	}

	return NewConversionResponse(&conversion), nil
}
