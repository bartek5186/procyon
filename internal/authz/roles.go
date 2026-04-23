package authz

import (
	"strings"

	ory "github.com/ory/client-go"
)

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

func RoleFromSession(session *ory.Session) string {
	if session == nil || session.Identity == nil {
		return RoleUser
	}

	if role, ok := roleFromMap(session.Identity.MetadataPublic); ok {
		return role
	}
	if role, ok := roleFromAny(session.Identity.Traits); ok {
		return role
	}
	if role, ok := roleFromMap(session.Identity.MetadataAdmin); ok {
		return role
	}

	return RoleUser
}

func roleFromMap(values map[string]interface{}) (string, bool) {
	if len(values) == 0 {
		return "", false
	}
	if role, ok := roleFromValue(values["role"]); ok {
		return role, true
	}
	if role, ok := roleFromValue(values["roles"]); ok {
		return role, true
	}
	return "", false
}

func roleFromAny(value interface{}) (string, bool) {
	switch typed := value.(type) {
	case map[string]interface{}:
		return roleFromMap(typed)
	default:
		return roleFromValue(typed)
	}
}

func roleFromValue(value interface{}) (string, bool) {
	switch typed := value.(type) {
	case string:
		role, ok := NormalizeRole(typed)
		return role, ok
	case []string:
		for _, item := range typed {
			if role, ok := NormalizeRole(item); ok {
				return role, true
			}
		}
	case []interface{}:
		for _, item := range typed {
			if role, ok := roleFromValue(item); ok {
				return role, true
			}
		}
	}

	return "", false
}
