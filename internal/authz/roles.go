package authz

import (
	"fmt"
	"strings"
)

const (
	RoleUser  = "user"
	RoleAdmin = "admin"
)

var selfAssignableRoles = map[string]bool{}

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

func IsSelfAssignableRole(role string) bool {
	return selfAssignableRoles[role]
}

func NormalizeSelfAssignableRoles(rawRoles []string) ([]string, error) {
	out := make([]string, 0, len(rawRoles))
	for _, raw := range rawRoles {
		role, ok := NormalizeRole(raw)
		if !ok {
			return nil, fmt.Errorf("unknown role: %q", raw)
		}
		if !IsSelfAssignableRole(role) {
			return nil, fmt.Errorf("role %q cannot be self-assigned", role)
		}
		out = append(out, role)
	}
	return out, nil
}
