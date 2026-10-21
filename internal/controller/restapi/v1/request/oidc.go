package request

// OIDCCallback is what the provider sent back to the web client's callback route.
type OIDCCallback struct {
	Code  string `json:"code"  minLength:"1" doc:"The code query parameter of the callback"`
	State string `json:"state" minLength:"1" doc:"The state query parameter of the callback"`
}
