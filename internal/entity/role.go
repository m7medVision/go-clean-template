package entity

import "slices"

// Role is what a caller may do beyond acting on their own records.
type Role string

const (
	RoleAdmin Role = "admin"
	RoleUser  Role = "user"
)

// Valid reports whether r is a known role.
func (r Role) Valid() bool {
	switch r {
	case RoleAdmin, RoleUser:
		return true
	default:
		return false
	}
}

// ParseRoles keeps the known roles in names, in order and without duplicates. A token can carry any
// string, so unknown names are dropped rather than rejected.
func ParseRoles(names []string) []Role {
	roles := make([]Role, 0, len(names))

	for _, name := range names {
		r := Role(name)
		if r.Valid() && !slices.Contains(roles, r) {
			roles = append(roles, r)
		}
	}

	return roles
}

// LocalRoles are the roles a sign-in the app verifies itself grants, such as a password. Admin
// comes only from an identity provider.
func LocalRoles() []Role {
	return []Role{RoleUser}
}

// Principal is the authenticated caller: who they are and the roles their sign-in granted.
type Principal struct {
	UserID string
	Roles  []Role
}

// Has reports whether the caller holds role r.
func (p Principal) Has(r Role) bool {
	return slices.Contains(p.Roles, r)
}

// RoleNames is roles as the strings an auth token carries.
func RoleNames(roles []Role) []string {
	names := make([]string, len(roles))

	for i, r := range roles {
		names[i] = string(r)
	}

	return names
}
