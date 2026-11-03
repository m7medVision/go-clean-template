package entity

import "time"

// ConversionStreamID is the stream of the conversion with id.
func ConversionStreamID(id string) string {
	return "conversion-" + id
}

// ConversionAggregate is one conversion on the write side. A conversion never changes once made,
// so its stream holds its one event.
type ConversionAggregate struct {
	AggregateBase

	id         string
	conversion Conversion
}

// NewConversionAggregate records userID's conversion c, validated and with its rate applied.
func NewConversionAggregate(id, userID string, c *Conversion, at time.Time) *ConversionAggregate {
	a := &ConversionAggregate{}
	Record(a, CurrencyConverted{
		ConversionID: id, UserID: userID, From: c.From, To: c.To,
		Amount: c.Amount, Rate: c.Rate, Result: c.Result, RateDate: c.RateDate, At: at,
	})

	return a
}

// ImportConversionAggregate records a conversion stored before conversions stored events.
func ImportConversionAggregate(id, userID string, c *Conversion, createdAt time.Time) *ConversionAggregate {
	a := &ConversionAggregate{}
	Record(a, ConversionImported{
		ConversionID: id, UserID: userID, From: c.From, To: c.To,
		Amount: c.Amount, Rate: c.Rate, Result: c.Result, RateDate: c.RateDate, CreatedAt: createdAt,
	})

	return a
}

// StreamID -.
func (a *ConversionAggregate) StreamID() string {
	return ConversionStreamID(a.id)
}

// State is the conversion.
func (a *ConversionAggregate) State() Conversion {
	return a.conversion
}

// Apply -.
func (a *ConversionAggregate) Apply(e Event) {
	switch ev := e.(type) {
	case CurrencyConverted:
		a.id = ev.ConversionID
		a.conversion = Conversion{
			From: ev.From, To: ev.To, Amount: ev.Amount, Rate: ev.Rate, Result: ev.Result, RateDate: ev.RateDate,
		}
	case ConversionImported:
		a.id = ev.ConversionID
		a.conversion = Conversion{
			From: ev.From, To: ev.To, Amount: ev.Amount, Rate: ev.Rate, Result: ev.Result, RateDate: ev.RateDate,
		}
	}
}
