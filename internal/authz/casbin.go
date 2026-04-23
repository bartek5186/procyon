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
	e  *casbin.Enforcer
	mu sync.Mutex
}

func NewCasbinAuthorizer(db *gorm.DB) (*CasbinAuthorizer, error) {
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

	out := &CasbinAuthorizer{e: e}
	if err := out.ensureBasePolicy(); err != nil {
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

func (a *CasbinAuthorizer) Can(_ context.Context, userID, obj, act string) (bool, error) {
	userID = strings.TrimSpace(userID)
	obj = strings.TrimSpace(obj)
	act = strings.TrimSpace(act)

	if userID == "" || obj == "" || act == "" {
		return false, nil
	}

	return a.e.Enforce(userID, obj, act)
}

func (a *CasbinAuthorizer) EnsureUserRole(ctx context.Context, userID, role string) error {
	return a.SetUserRole(ctx, userID, role)
}

func (a *CasbinAuthorizer) SetUserRole(_ context.Context, userID, role string) error {
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return nil
	}

	normalizedRole := NormalizeRoleOrDefault(role)

	a.mu.Lock()
	defer a.mu.Unlock()

	currentRoles, err := a.e.GetRolesForUser(userID)
	if err != nil {
		return err
	}

	changed := false
	hasTarget := false
	for _, currentRole := range currentRoles {
		if currentRole == normalizedRole {
			hasTarget = true
			continue
		}

		ok, err := a.e.DeleteRoleForUser(userID, currentRole)
		if err != nil {
			return err
		}
		if ok {
			changed = true
		}
	}

	if !hasTarget {
		ok, err := a.e.AddRoleForUser(userID, normalizedRole)
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
