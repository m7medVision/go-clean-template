package analytics

type SignUpMethod string

const (
	SignUpPassword SignUpMethod = "password"
	SignUpOIDC     SignUpMethod = "oidc"
)

type UserRegistered struct {
	Method SignUpMethod `json:"method"`
}

func (UserRegistered) Name() string { return "user_registered" }

func userEvents() []Properties {
	return []Properties{UserRegistered{}}
}
