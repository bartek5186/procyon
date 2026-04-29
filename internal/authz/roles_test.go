package authz

import "testing"

func TestNormalizeRole(t *testing.T) {
	tests := map[string]string{
		"user":    RoleUser,
		"member":  RoleUser,
		"regular": RoleUser,
		"admin":   RoleAdmin,
		" ADMIN ": RoleAdmin,
	}

	for raw, want := range tests {
		got, ok := NormalizeRole(raw)
		if !ok {
			t.Fatalf("expected %q to normalize", raw)
		}
		if got != want {
			t.Fatalf("unexpected normalized role for %q: got %q want %q", raw, got, want)
		}
	}
}

func TestNormalizeRoleRejectsUnknown(t *testing.T) {
	if role, ok := NormalizeRole("owner"); ok || role != "" {
		t.Fatalf("expected unknown role to be rejected, got role=%q ok=%v", role, ok)
	}
}
