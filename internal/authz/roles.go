package authz

import "strings"

const (
	RoleUser  = "user"
	RoleAdmin = "admin"
)

func NormalizeRole(raw string) (string, bool) {
	role := strings.ToLower(strings.TrimSpace(raw))
	switch role {
	case RoleUser, "account", "regular", "member":
		return RoleUser, true
	case RoleAdmin:
		return RoleAdmin, true
	default:
		return "", false
	}
}

func NormalizeRoleOrDefault(raw string) string {
	role, ok := NormalizeRole(raw)
	if !ok {
		return RoleUser
	}
	return role
}
