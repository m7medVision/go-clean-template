package request

// Register -.
type Register struct {
	Username string `json:"username" minLength:"3" maxLength:"255" example:"johndoe"`
	Email    string `json:"email" format:"email" minLength:"1" example:"john@example.com"`
	Password string `json:"password" minLength:"6" example:"secret123"`
}

// Login -.
type Login struct {
	Email    string `json:"email" format:"email" minLength:"1" example:"john@example.com"`
	Password string `json:"password" minLength:"1" example:"secret123"`
}
