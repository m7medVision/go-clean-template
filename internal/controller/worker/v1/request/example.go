package request

// Example is the payload of a message on the example queue.
type Example struct {
	Text string `json:"text"`
}
