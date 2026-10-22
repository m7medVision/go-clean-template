package entity

// WelcomeEmail is sent once a user registers.
type WelcomeEmail struct {
	To       string `json:"to"`
	Username string `json:"username"`
	LoginURL string `json:"login_url"`
}

// Template -.
func (WelcomeEmail) Template() string { return "welcome" }

// Recipient -.
func (e WelcomeEmail) Recipient() string { return e.To }
