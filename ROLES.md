# Roles And RBAC

This project uses Kratos for authentication and Casbin for authorization.

The important rule:

```text
Kratos identity ID -> Casbin/app DB -> role/policy
```

Kratos confirms who the user is. Casbin decides what that user can do.

The app does not trust role fields from Kratos `traits`, `metadata_public`, or `metadata_admin`.

## Configuration

RBAC is configured in `config/config.json`:

```json
{
  "auth": {
    "enabled": true,
    "provider": "kratos",
    "domain": "http://127.0.0.1:4433"
  },
  "rbac": {
    "enabled": true,
    "default_role": "user",
    "admin_identity_ids": [
      "kratos-identity-id-of-first-admin"
    ]
  }
}
```

Fields:

- `rbac.enabled` enables Casbin checks for protected routes.
- `rbac.default_role` is assigned to an authenticated identity that has no Casbin role yet.
- `rbac.admin_identity_ids` bootstraps initial admin identities into Casbin.

Use the Kratos identity ID, not email or username, in `admin_identity_ids`.

## Where Policies Live

Casbin uses the app database through `gorm-adapter`.

Policies and user-role links are persisted in Casbin's table, usually named:

```text
casbin_rule
```

The app seeds base policies on startup when RBAC is enabled.

Current base policies live in `internal/authz/casbin.go`:

```go
var defaultPolicies = [][]string{
    {RoleUser, "hello", "read"},
    {RoleAdmin, "hello", "manage"},
    {RoleAdmin, "admin", "manage"},
}

var defaultRoleHierarchy = [][]string{
    {RoleAdmin, RoleUser},
}
```

This means:

- `user` can `read` `hello`
- `admin` can `manage` `hello`
- `admin` can `manage` `admin`
- `admin` inherits `user`

## Adding A Role

Add the role constant in `internal/authz/roles.go`:

```go
const (
    RoleUser    = "user"
    RoleAdmin   = "admin"
    RoleManager = "manager"
)
```

Then allow it in `NormalizeRole`:

```go
func NormalizeRole(raw string) (string, bool) {
    role := strings.ToLower(strings.TrimSpace(raw))
    switch role {
    case RoleUser, "account", "regular", "member":
        return RoleUser, true
    case RoleAdmin:
        return RoleAdmin, true
    case RoleManager:
        return RoleManager, true
    default:
        return "", false
    }
}
```

Add policies in `internal/authz/casbin.go`:

```go
var defaultPolicies = [][]string{
    {RoleUser, "hello", "read"},
    {RoleManager, "reports", "read"},
    {RoleAdmin, "hello", "manage"},
    {RoleAdmin, "reports", "manage"},
    {RoleAdmin, "admin", "manage"},
}
```

Optionally add hierarchy:

```go
var defaultRoleHierarchy = [][]string{
    {RoleManager, RoleUser},
    {RoleAdmin, RoleManager},
}
```

## Protecting Routes

Routes are protected in `main.go` with:

```go
secured.GET("/hello", helloController.HelloAuthenticated, rbac.Require("hello", "read"))
securedAdmin.GET("/hello", helloController.HelloAdmin, rbac.Require("hello", "manage"))
```

Pattern:

```go
rbac.Require("<object>", "<action>")
```

Examples:

```go
rbac.Require("reports", "read")
rbac.Require("reports", "manage")
rbac.Require("payments", "refund")
rbac.Require("admin", "manage")
```

Keep object/action names stable and small. Avoid dynamic names such as IDs in the policy object.

## Assigning Roles

For the first admin, use config:

```json
{
  "rbac": {
    "admin_identity_ids": [
      "kratos-identity-id-of-first-admin"
    ]
  }
}
```

For application-managed role assignment, call the authorizer from an admin-only flow:

```go
if err := authorizer.AddUserRole(ctx, identityID, authz.RoleAdmin); err != nil {
    return err
}
```

Do this from an endpoint protected by an existing admin policy, not from a public route.

## Request Flow

1. `KratosAuth` verifies the request with Kratos.
2. The middleware stores the Kratos session in Echo context.
3. `CasbinRBAC` reads `session.Identity.Id`.
4. If the identity has no Casbin role, `rbac.default_role` is assigned.
5. Casbin checks `identity_id -> role -> policy`.
6. The handler runs only if the policy allows the action.

## Security Notes

- Do not trust `traits.role`.
- Do not trust `metadata_public.role`.
- Do not use email as the RBAC subject.
- Use Kratos identity ID as the Casbin subject.
- Bootstrap only known admin identity IDs.
- Keep role assignment behind admin-only endpoints.
