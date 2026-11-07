package entity

import (
	"time"

	"github.com/shopspring/decimal"
)

// CurrencyConverted is a conversion a user made at a reference rate.
type CurrencyConverted struct {
	ConversionID string          `json:"conversion_id"`
	UserID       string          `json:"user_id"`
	From         string          `json:"from"`
	To           string          `json:"to"`
	Amount       decimal.Decimal `json:"amount"`
	Rate         decimal.Decimal `json:"rate"`
	Result       decimal.Decimal `json:"result"`
	RateDate     time.Time       `json:"rate_date"`
	At           time.Time       `json:"at"`
}

// ConversionImported is a conversion stored before conversions stored events: the backfill's one
// event per exchange_history row.
type ConversionImported struct {
	ConversionID string          `json:"conversion_id"`
	UserID       string          `json:"user_id"`
	From         string          `json:"from"`
	To           string          `json:"to"`
	Amount       decimal.Decimal `json:"amount"`
	Rate         decimal.Decimal `json:"rate"`
	Result       decimal.Decimal `json:"result"`
	RateDate     time.Time       `json:"rate_date"`
	CreatedAt    time.Time       `json:"created_at"`
}

// EventType -.
func (CurrencyConverted) EventType() string { return "CurrencyConverted" }

// SchemaVersion is the version of the event's shape, raised with an upcast when it changes.
func (CurrencyConverted) SchemaVersion() int { return 1 }

// EventType -.
func (ConversionImported) EventType() string { return "ConversionImported" }

// SchemaVersion -.
func (ConversionImported) SchemaVersion() int { return 1 }

// ConversionEvents lists the conversion events, one zero value each.
func ConversionEvents() []Event {
	return []Event{CurrencyConverted{}, ConversionImported{}}
}
