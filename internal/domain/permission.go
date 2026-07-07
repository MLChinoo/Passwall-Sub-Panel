package domain

import "time"

// Permission is a fine-grained capability, always in "resource.action" form.
// It is the single vocabulary the RBAC model is built on: route gates
// (RequirePermission), seeded and custom role bundles, per-user overrides, the
// SPA capability picker, and SSO role validation all index into this catalog.
//
// Permissions serialize straight into JSON columns and the session payload, so
// the string VALUE is the stable contract — rename a const, never the value.
type Permission string

const (
	// ---- users ----
	PermUsersRead    Permission = "users.read"    // list/get accounts, their rules & passkey metadata
	PermUsersWrite   Permission = "users.write"   // create/edit accounts; resets; enable-disable; unlink SSO; edit rules
	PermUsersDelete  Permission = "users.delete"  // delete accounts
	PermUsersElevate Permission = "users.elevate" // assign roles / per-user grants; act on privileged targets

	// ---- traffic ----
	PermTrafficRead  Permission = "traffic.read"  // per-user traffic reports/history/top
	PermTrafficWrite Permission = "traffic.write" // on-demand poll; set/reset a user's usage counters
	// PermTrafficNodesRead gates NODE-aggregated traffic. Deliberately split out
	// of traffic.read and marked global-only: a node aggregates users across all
	// groups, so there is no group_id axis a scoped actor could be filtered to.
	PermTrafficNodesRead Permission = "traffic.nodes.read"

	// ---- groups ----
	PermGroupsRead  Permission = "groups.read"  // list/get groups (needed to pick a group)
	PermGroupsWrite Permission = "groups.write" // group CRUD, render layout, per-group scope-settings

	// ---- nodes ----
	PermNodesRead   Permission = "nodes.read"   // list/get nodes (secrets redacted), separators, unmanaged
	PermNodesToggle Permission = "nodes.toggle" // ONLY enable/disable an existing node
	PermNodesWrite  Permission = "nodes.write"  // node/inbound/separator CRUD; reveals inbound secret blobs

	// ---- integrations / infra (all global break-glass) ----
	PermPanelsWrite    Permission = "panels.write" // 3X-UI panel CRUD, probe, upgrade, web-cert
	PermCertsWrite     Permission = "certs.write"  // ACME certs, DNS creds, CA accounts (private keys)
	PermContentRead    Permission = "content.read" // read subscription templates, rulesets, locale packs
	PermTemplatesWrite Permission = "templates.write"
	PermRulesetsWrite  Permission = "rulesets.write"
	PermSettingsWrite  Permission = "settings.write" // system settings, GeoIP, mail
	PermSSOWrite       Permission = "sso.write"      // SAML/OIDC config incl. role-mapping rules
	PermSyncOperate    Permission = "sync.operate"   // sync-task list/retry/cancel/purge, reconcile-run
	PermAuditRead      Permission = "audit.read"     // audit log, auth-events, sub/email logs, dashboard, alerts
	PermAuditClear     Permission = "audit.clear"    // clear/purge audit, sub-logs, email-logs
	PermRolesWrite     Permission = "roles.write"    // manage the role catalog + assignments + overrides

	// PermAll is the wildcard held ONLY by the immutable Global Administrator.
	// Has()/Superset() treat it as a superset of the whole catalog, so a future
	// permission is auto-covered with no data migration. It is NEVER a member of
	// AllPermissions and is rejected at write time in any non-immutable role, so
	// the sole "*" holder is always the GA that the last-admin floor protects.
	PermAll Permission = "*"
)

// AllPermissions is the canonical, ordered catalog EXCLUDING the wildcard. It
// drives payload validation, the SPA picker, and "*" expansion. Order is stable
// (grouped by resource) so the UI renders deterministically.
var AllPermissions = []Permission{
	PermUsersRead, PermUsersWrite, PermUsersDelete, PermUsersElevate,
	PermTrafficRead, PermTrafficWrite, PermTrafficNodesRead,
	PermGroupsRead, PermGroupsWrite,
	PermNodesRead, PermNodesToggle, PermNodesWrite,
	PermPanelsWrite, PermCertsWrite,
	PermContentRead, PermTemplatesWrite, PermRulesetsWrite,
	PermSettingsWrite, PermSSOWrite,
	PermSyncOperate, PermAuditRead, PermAuditClear,
	PermRolesWrite,
}

// groupScopeablePerms is the exact set that carries a target group_id axis
// (Entra "administrative unit" scoping). A scoped role assignment or override
// may only carry these — anything else is tenant-wide and rejected when scoped.
var groupScopeablePerms = map[Permission]struct{}{
	PermUsersRead: {}, PermUsersWrite: {}, PermUsersDelete: {}, PermUsersElevate: {},
	PermTrafficRead: {}, PermTrafficWrite: {},
	PermGroupsRead: {}, PermGroupsWrite: {},
}

// GroupScopeable reports whether p can be granted scoped to specific groups.
// The wildcard and every infra/global permission return false.
func (p Permission) GroupScopeable() bool {
	_, ok := groupScopeablePerms[p]
	return ok
}

// IsManagement reports whether holding p grants power beyond a pure end-user.
// Every real catalog permission (and the wildcard) is a management permission;
// only the absence of a permission (the empty string) is not. Drives staff-2FA
// enforcement, staff-alert recipients, and the admin-console landing decision —
// so those key off effective permissions, never the literal admin/operator role.
func (p Permission) IsManagement() bool {
	return p != ""
}

// RoleDef is a named role: a bundle of permissions. domain.Role stays the
// string slug that users.role stores; RoleDef is the definition behind it.
type RoleDef struct {
	Slug        string
	Name        string
	Description string
	Permissions []Permission
	// Builtin roles cannot be deleted. Immutable roles additionally cannot be
	// edited and are pinned to their canonical permission set (only the Global
	// Administrator is immutable, with Permissions == ["*"]).
	Builtin   bool
	Immutable bool
	CreatedAt time.Time
	UpdatedAt time.Time
}

// HasWildcard reports whether the role holds "*" (i.e. is the Global
// Administrator definition).
func (r *RoleDef) HasWildcard() bool {
	for _, p := range r.Permissions {
		if p == PermAll {
			return true
		}
	}
	return false
}

// PermissionEffect is the direction of a per-user override. Deny always beats
// grant when both apply (except for the wildcard-holding GA, where denies are
// ignored entirely).
type PermissionEffect string

const (
	EffectGrant PermissionEffect = "grant"
	EffectDeny  PermissionEffect = "deny"
)

// PermissionOverride is a per-user grant or deny layered on top of the user's
// role(s). An empty ScopeGroupIDs means the override applies tenant-wide;
// otherwise it applies only within those groups.
type PermissionOverride struct {
	Permission    Permission       `json:"permission"`
	Effect        PermissionEffect `json:"effect"`
	ScopeGroupIDs []int64          `json:"scope_group_ids,omitempty"`
}

// RoleAssignment is an ADDITIONAL role granted to a user beyond their primary
// users.role, optionally limited to a set of groups (Entra administrative
// unit). Empty ScopeGroupIDs means the additional role applies tenant-wide.
type RoleAssignment struct {
	ID            int64
	UserID        int64
	RoleSlug      string
	ScopeGroupIDs []int64
	// Origin is provenance: "admin" for a hand-granted assignment, or
	// "sso:<provider>" for one an IdP rule created. SSO reconcile is authoritative
	// only over its own sso:* rows and never touches an admin's hand-grant.
	Origin    string
	CreatedAt time.Time
}

// Assignment origins.
const (
	AssignmentOriginAdmin = "admin"
)
