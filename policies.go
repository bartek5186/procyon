package main

import "github.com/bartek5186/procyon-core/authz"

var applicationPolicies = []authz.Policy{
	// procyon:module-user-policies
	{Role: authz.RoleUser, Domain: "*", Object: "hello", Action: "read"},
	// procyon:module-admin-policies
	{Role: authz.RoleAdmin, Domain: "*", Object: "hello", Action: "manage"},
}
