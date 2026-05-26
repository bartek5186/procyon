package authz

import (
	"context"
	"strings"
	"sync"

	"github.com/casbin/casbin/v3"
	"github.com/casbin/casbin/v3/model"
	gormadapter "github.com/casbin/gorm-adapter/v3"
	"gorm.io/gorm"
)

const casbinModel = `
[request_definition]
r = sub, dom, obj, act

[policy_definition]
p = role, dom, obj, act, eft

[role_definition]
g = _, _

[policy_effect]
e = some(where (p.eft == allow)) && !some(where (p.eft == deny))

[matchers]
m = g(r.sub, p.role) \
    && keyMatch(r.dom, p.dom) \
    && keyMatch(r.obj, p.obj) \
    && keyMatch(r.act, p.act) \
    && domainAccess(r.sub, r.dom, r.obj, r.act)
`

var defaultPolicies = [][]string{
	{RoleUser, "*", "hello", "read", "allow"},
	{RoleAdmin, "*", "hello", "manage", "allow"},
	{RoleAdmin, "*", "*", "*", "allow"},
}

type CasbinAuthorizer struct {
	e           *casbin.Enforcer
	mu          sync.Mutex
	defaultRole string
}

func NewCasbinAuthorizer(db *gorm.DB, defaultRole string, adminIdentityIDs []string) (*CasbinAuthorizer, error) {
	adapter, err := gormadapter.NewAdapterByDB(db)
	if err != nil {
		return nil, err
	}

	m, err := model.NewModelFromString(casbinModel)
	if err != nil {
		return nil, err
	}

	e, err := casbin.NewEnforcer(m, adapter)
	if err != nil {
		return nil, err
	}

	// TODO: zastąpić prawdziwą logiką sprawdzania własności domeny (player:{id}, club:{id})
	e.AddFunction("domainAccess", func(args ...interface{}) (interface{}, error) {
		return true, nil
	})

	if err := e.LoadPolicy(); err != nil {
		return nil, err
	}

	out := &CasbinAuthorizer{
		e:           e,
		defaultRole: NormalizeRoleOrDefault(defaultRole),
	}
	if err := out.ensureBasePolicy(); err != nil {
		return nil, err
	}
	if err := out.ensureBootstrapAdmins(adminIdentityIDs); err != nil {
		return nil, err
	}

	return out, nil
}

func (a *CasbinAuthorizer) ensureBasePolicy() error {
	a.mu.Lock()
	defer a.mu.Unlock()

	changed := false

	for _, policy := range defaultPolicies {
		ok, err := a.e.AddPolicy(policy)
		if err != nil {
			return err
		}
		if ok {
			changed = true
		}
	}

	if changed {
		return a.e.SavePolicy()
	}
	return nil
}

func (a *CasbinAuthorizer) ensureBootstrapAdmins(identityIDs []string) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	changed := false
	for _, identityID := range identityIDs {
		identityID = strings.TrimSpace(identityID)
		if identityID == "" {
			continue
		}

		ok, err := a.e.AddRoleForUser(identityID, RoleAdmin)
		if err != nil {
			return err
		}
		if ok {
			changed = true
		}
	}

	if changed {
		return a.e.SavePolicy()
	}
	return nil
}

func (a *CasbinAuthorizer) Can(_ context.Context, userID, dom, obj, act string) (bool, error) {
	userID = strings.TrimSpace(userID)
	dom = strings.TrimSpace(dom)
	obj = strings.TrimSpace(obj)
	act = strings.TrimSpace(act)

	if userID == "" || dom == "" || obj == "" || act == "" {
		return false, nil
	}

	return a.e.Enforce(userID, dom, obj, act)
}

func (a *CasbinAuthorizer) EnsureDefaultRole(_ context.Context, userID string) (string, error) {
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return "", nil
	}

	a.mu.Lock()
	defer a.mu.Unlock()

	currentRoles, err := a.e.GetRolesForUser(userID)
	if err != nil {
		return "", err
	}
	fallbackRole := ""
	for _, currentRole := range currentRoles {
		if role, ok := NormalizeRole(currentRole); ok {
			if role == RoleAdmin {
				return role, nil
			}
			if fallbackRole == "" {
				fallbackRole = role
			}
		}
	}
	if fallbackRole != "" {
		return fallbackRole, nil
	}

	role := a.defaultRole
	if role == "" {
		return "", nil
	}
	ok, err := a.e.AddRoleForUser(userID, role)
	if err != nil {
		return "", err
	}
	if ok {
		return role, a.e.SavePolicy()
	}

	return role, nil
}

func (a *CasbinAuthorizer) AddUserRole(_ context.Context, userID, role string) error {
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return nil
	}
	role = NormalizeRoleOrDefault(role)

	a.mu.Lock()
	defer a.mu.Unlock()

	ok, err := a.e.AddRoleForUser(userID, role)
	if err != nil {
		return err
	}
	if ok {
		return a.e.SavePolicy()
	}
	return nil
}

func (a *CasbinAuthorizer) HasUserRole(_ context.Context, userID, role string) (bool, error) {
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return false, nil
	}
	role, ok := NormalizeRole(role)
	if !ok {
		return false, nil
	}

	a.mu.Lock()
	defer a.mu.Unlock()

	return a.e.HasRoleForUser(userID, role)
}

func (a *CasbinAuthorizer) SetUserSelfSelectedRoles(_ context.Context, userID string, rawRoles []string) ([]string, error) {
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return nil, nil
	}

	roles, err := NormalizeSelfAssignableRoles(rawRoles)
	if err != nil {
		return nil, err
	}

	selected := make(map[string]bool, len(roles))
	for _, role := range roles {
		selected[role] = true
	}

	a.mu.Lock()
	defer a.mu.Unlock()

	changed := false

	ok, err := a.e.AddRoleForUser(userID, RoleUser)
	if err != nil {
		return nil, err
	}
	if ok {
		changed = true
	}

	currentRoles, err := a.e.GetRolesForUser(userID)
	if err != nil {
		return nil, err
	}
	for _, currentRole := range currentRoles {
		role, ok := NormalizeRole(currentRole)
		if !ok || !IsSelfAssignableRole(role) || selected[role] {
			continue
		}
		ok, err := a.e.DeleteRoleForUser(userID, role)
		if err != nil {
			return nil, err
		}
		if ok {
			changed = true
		}
	}

	for _, role := range roles {
		ok, err := a.e.AddRoleForUser(userID, role)
		if err != nil {
			return nil, err
		}
		if ok {
			changed = true
		}
	}

	if changed {
		if err := a.e.SavePolicy(); err != nil {
			return nil, err
		}
	}

	currentRoles, err = a.e.GetRolesForUser(userID)
	if err != nil {
		return nil, err
	}
	return normalizedRolesForOutput(currentRoles), nil
}

func (a *CasbinAuthorizer) GetUserRoles(ctx context.Context, userID string) ([]string, error) {
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return nil, nil
	}

	if _, err := a.EnsureDefaultRole(ctx, userID); err != nil {
		return nil, err
	}

	a.mu.Lock()
	defer a.mu.Unlock()

	currentRoles, err := a.e.GetRolesForUser(userID)
	if err != nil {
		return nil, err
	}
	return normalizedRolesForOutput(currentRoles), nil
}

func normalizedRolesForOutput(rawRoles []string) []string {
	seen := make(map[string]bool, len(rawRoles))
	for _, raw := range rawRoles {
		role, ok := NormalizeRole(raw)
		if ok {
			seen[role] = true
		}
	}

	preferredOrder := []string{
		RoleUser,
		RoleAdmin,
	}
	out := make([]string, 0, len(seen))
	for _, role := range preferredOrder {
		if seen[role] {
			out = append(out, role)
		}
	}
	return out
}
