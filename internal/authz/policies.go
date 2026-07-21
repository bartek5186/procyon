package authz

import coreauthz "github.com/bartek5186/procyon-core/authz"

var applicationPolicies = []coreauthz.Policy{
	// procyon:module-user-policies
	{Role: coreauthz.RoleUser, Domain: "*", Object: "hello", Action: "read"},
	// procyon:module-admin-policies
	{Role: coreauthz.RoleAdmin, Domain: "*", Object: "hello", Action: "manage"},
}

// Policies returns a defensive copy of the application's RBAC policies.
func Policies() []coreauthz.Policy {
	return append([]coreauthz.Policy(nil), applicationPolicies...)
}
