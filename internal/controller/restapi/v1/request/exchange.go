package request

// Convert -. Amounts are decimal strings so they never pass through a float; the controller parses
// them into decimal.Decimal. Code case and "from != to" are domain rules, checked by the entity.
type Convert struct {
	_ struct{} `additionalProperties:"true"`

	From   string `json:"from" pattern:"^[A-Za-z]{3}$" example:"EUR"`
	To     string `json:"to" pattern:"^[A-Za-z]{3}$" example:"USD"`
	Amount string `json:"amount" pattern:"^[0-9]{1,15}(\\.[0-9]{1,8})?$" example:"100.10"`
}
