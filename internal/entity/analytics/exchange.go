package analytics

type CurrencyConverted struct {
	From string `json:"from"`
	To   string `json:"to"`
}

func (CurrencyConverted) Name() string { return "currency_converted" }

func exchangeEvents() []Properties {
	return []Properties{CurrencyConverted{}}
}
