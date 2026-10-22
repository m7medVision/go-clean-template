package config

// OIDC -. SSO sign-in through an OpenID Connect provider, on when Issuer is set. The provider
// redirects back to APP_BASE_URL/sso/callback, which must be a redirect URI of the client.
type oidc struct {
	Issuer       string `env:"OIDC_ISSUER"`
	ClientID     string `env:"OIDC_CLIENT_ID"`
	ClientSecret string `env:"OIDC_CLIENT_SECRET"`
	// RolesClaim is the dotted path to the ID-token claim holding the provider's role names:
	// realm_access.roles on Keycloak, roles on Entra ID, groups on Okta.
	RolesClaim string `env:"OIDC_ROLES_CLAIM" envDefault:"realm_access.roles"`
	// RoleMap maps provider role names to app roles (admin, user) as name:role pairs. A provider
	// role not in the map grants nothing; a sign-in that maps to no role gets user.
	RoleMap map[string]string `env:"OIDC_ROLE_MAP" envSeparator:"," envKeyValSeparator:":"`
}
