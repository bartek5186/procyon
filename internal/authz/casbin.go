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
r = sub, obj, act

[policy_definition]
p = sub, obj, act

[role_definition]
g = _, _

[policy_effect]
e = some(where (p.eft == allow))

[matchers]
m = g(r.sub, p.sub) && r.obj == p.obj && r.act == p.act
`

var defaultPolicies = [][]string{
	{RoleUser, "hello", "read"},
	{RoleAdmin, "hello", "manage"},
	{RoleAdmin, "admin", "manage"},
}

var defaultRoleHierarchy = [][]string{
	{RoleAdmin, RoleUser},
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

	for _, link := range defaultRoleHierarchy {
		ok, err := a.e.AddGroupingPolicy(link)
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

func (a *CasbinAuthorizer) Can(_ context.Context, userID, obj, act string) (bool, error) {
	userID = strings.TrimSpace(userID)
	obj = strings.TrimSpace(obj)
	act = strings.TrimSpace(act)

	if userID == "" || obj == "" || act == "" {
		return false, nil
	}

	return a.e.Enforce(userID, obj, act)
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
