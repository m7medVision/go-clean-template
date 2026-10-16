package request

// Translate -.
type Translate struct {
	Source      string `json:"source" minLength:"1" example:"auto"`
	Destination string `json:"destination" minLength:"1" example:"en"`
	Original    string `json:"original" minLength:"1" example:"текст для перевода"`
}
