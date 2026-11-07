package exchange

import (
	"time"

	"github.com/m7medvision/go-clean-template/internal/controller/graphql/model"
	"github.com/m7medvision/go-clean-template/internal/entity"
)

// newConversionList carries the history's decimals as strings, so no precision is lost to a
// floating-point JSON number.
func newConversionList(history entity.List[entity.Conversion]) *model.ConversionList {
	conversions := make([]model.Conversion, len(history.Items))
	for i, c := range history.Items {
		conversions[i] = model.Conversion{
			From:     c.From,
			To:       c.To,
			Amount:   c.Amount.String(),
			Rate:     c.Rate.String(),
			Result:   c.ResultString(),
			RateDate: c.RateDate.Format(time.DateOnly),
		}
	}

	return &model.ConversionList{Conversions: conversions, Total: history.Total}
}
