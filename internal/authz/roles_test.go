package authz

import (
	"testing"

	ory "github.com/ory/client-go"
)

func TestRoleFromSessionUsesMetadataPublicFirst(t *testing.T) {
	session := &ory.Session{
		Identity: &ory.Identity{
			Id: "user-1",
			MetadataPublic: map[string]interface{}{
				"role": "admin",
			},
			Traits: map[string]interface{}{
				"role": "user",
			},
		},
	}

	if got, want := RoleFromSession(session), RoleAdmin; got != want {
		t.Fatalf("unexpected role: got %q want %q", got, want)
	}
}

func TestRoleFromSessionFallsBackToUser(t *testing.T) {
	session := &ory.Session{
		Identity: &ory.Identity{
			Id:     "user-2",
			Traits: map[string]interface{}{"name": "No role"},
		},
	}

	if got, want := RoleFromSession(session), RoleUser; got != want {
		t.Fatalf("unexpected role: got %q want %q", got, want)
	}
}
