package entity

// WelcomeEmail is sent once a user registers.
type WelcomeEmail struct {
	To       string `json:"to"`
	Username string `json:"username"`
	LoginURL string `json:"login_url"`
}

func (WelcomeEmail) Template() string { return "welcome" }

func (e WelcomeEmail) Recipient() string { return e.To }
