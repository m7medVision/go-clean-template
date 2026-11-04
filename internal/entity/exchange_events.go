package entity

import (
	"time"

	"github.com/shopspring/decimal"
)

// The conversion events, all at schema version 1. Amounts are decimals, which encode to JSON as
// strings, so no amount passes through a float on its way to the store.
const _conversionEventsVersion = 1

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

// SchemaVersion -.
func (CurrencyConverted) SchemaVersion() int { return _conversionEventsVersion }

// EventType -.
func (ConversionImported) EventType() string { return "ConversionImported" }

// SchemaVersion -.
func (ConversionImported) SchemaVersion() int { return _conversionEventsVersion }

func conversionEvents() []Event {
	return []Event{CurrencyConverted{}, ConversionImported{}}
}
