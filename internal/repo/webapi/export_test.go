package webapi

import "time"

func (w *ExchangeWebAPI) SetTimeout(d time.Duration) {
	w.timeout = d
}
