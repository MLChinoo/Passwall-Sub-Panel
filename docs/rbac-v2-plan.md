# PSP RBAC v2 — Entra-ID-Style Custom Roles, Group-Scoped Assignments & SSO Role Mapping

**Status:** Buildable implementation plan (lead-architect synthesis of four discovery passes, hardened against an adversarial security review — every blocker/high/medium finding folded into the design below).
**Core decision (locked):** *Hybrid model, incremental migration.* `users.role` stays the single **primary role** slug (an FK-by-convention into a new `roles` table). The built-in Global Administrator **keeps the slug `"admin"`** — only its display name becomes "Global Administrator" — so every existing `User.Role='admin'` row, the `CountEnabledAdmins` (`WHERE role='admin'`) lockout math, the JWT `r` claim, the 60s live re-bind, and SSO rules emitting `admin`/`operator`/`user` migrate with **zero row changes**. Scoped grants and per-user overrides are *additive* layers on top. The full-rewrite alternative (demote `users.role`, move everything to assignments) is rejected: blast radius across JWT/last-admin/SSO/persisted frontend for no v1 benefit.

The JWT stays thin. Effective permissions are **re-resolved live** on the existing 60s per-user LRU cadence in `RequireAuth` — never embedded in the token.

**Error-classification convention (applies throughout — resolves review gap #9).** `errors.Is` callers and HTTP mapping stay consistent by a fixed rule:
- **`ErrValidation`** (→ 400/409) = the *request payload itself* is malformed or breaks a data invariant regardless of who asks: `*` in a non-immutable role, a scoped assignment carrying a global-only permission, editing an immutable role, deleting a referenced/builtin role, an SSO rule naming a non-existent slug.
- **`ErrForbidden`** (→ 403) = the payload is well-formed but *this actor* may not perform it: grant-only-what-you-hold violations, out-of-scope target, acting on a peer/superior, self-targeted RBAC writes, last-GA floor breach initiated by a non-GA.
- **`ErrNotFound`** (→ 404, or waved through in fail-closed guards) keeps its current meaning.
- Fail-closed infra errors (repo/DB failures inside a guard) → **503**, never "allow."

---

## 1. Overview & Entra-ID mapping

| Entra-ID concept | PSP construct | Backing store |
|---|---|---|
| **Global Administrator** (immutable, holds everything, can't be locked out) | Built-in role, slug `admin`, `permissions=["*"]`, `Immutable=true`, denies ignored, protected by extended last-admin floor, **self-healed on every boot** | `roles` row |
| **Directory role** (bundle of permissions) | Custom or built-in `RoleDef` = ordered set of fine-grained `Permission`s | `roles` table |
| **Built-in editable roles** | Seeded `operator` + `user` (editable, deletable-guarded) | `roles` rows |
| **Directory role assignment** | `users.role` = the one **primary, tenant-wide** role (the anchor) | `users.role` column |
| **Administrative Unit** (scope a role to a slice of the directory) | `role_assignments` row = *additional* role grant limited to a set of `group_id`s (empty = tenant-wide) | `role_assignments` table |
| **Per-object grant/deny** | `permission_overrides` on the user (grant/deny; **deny wins**; ignored for GA) | JSON column on `users` |
| **Privileged Role Administrator** (who may manage roles) | `roles.write` permission + the **grant-only-what-you-hold** invariant on *both* assignment and **role-definition edits** | permission gate |
| **"Extra ceremony to grant Global Admin"** | SSO cannot mint primary `admin` unless a default-off toggle is set; assigning `admin` requires being a GA; **`*` cannot appear in any editable role** | guards §5, §7 |

A "group-scoped admin" in Entra terms = a user whose **primary** role is `user` plus one `role_assignments` row `{role: <some management role>, scope:[groupA, groupB]}`. They manage only users/traffic/groups inside A and B and hold no tenant-wide power (so they never satisfy the GA recovery floor).

---

## 2. Permission catalog (finalized)

`resource.action` strings — serialize straight into JSON columns and the session payload. Group-scopeable = the check has a target `group_id` axis; global perms ignore scope.

| Key | Meaning | Group-scopeable? |
|---|---|---|
| `users.read` | List/get accounts, their rules & passkey metadata | ✅ |
| `users.write` | Create/edit accounts; reset creds/password/2FA/emergency; enable-disable; service-status; unlink SSO; edit user rules; revoke passkeys | ✅ |
| `users.delete` | Delete accounts | ✅ |
| `users.elevate` | Assign roles / per-user grants; act on staff-level (privileged) targets | ✅ (but **primary-role change always needs *global* `users.elevate`** — H4) |
| `traffic.read` | Per-user traffic reports/history/top | ✅ |
| `traffic.write` | On-demand poll; set/reset a user's usage counters | ✅ (poll itself is global) |
| `traffic.nodes.read` | **Node-aggregated** traffic (top/history across all groups) | ❌ **global-only** (H3) |
| `groups.read` | List/get groups (needed to pick a group) | ✅ |
| `groups.write` | Group CRUD, render layout, per-group scope-settings | ✅ (create is inherently global) |
| `nodes.read` | List/get nodes, inbound detail (secrets redacted), separators, unmanaged | ❌ |
| `nodes.toggle` | **Only** enable/disable an existing node (preserves today's operator-accessible `set-enabled` without granting secret-revealing writes) | ❌ |
| `nodes.write` | Node/inbound/separator CRUD, reorder, claim/import/detach/recreate, reality keypair, cert-source; **reveals** inbound secret blobs | ❌ |
| `panels.write` | 3X-UI panel (server) CRUD, probe, panel/xray upgrade, web-cert (break-glass) | ❌ |
| `certs.write` | ACME certs, DNS creds, CA accounts CRUD/renew/download (private keys) | ❌ |
| `content.read` | Read subscription templates, rulesets, locale packs | ❌ |
| `templates.write` | Template create/update/delete/reset | ❌ |
| `rulesets.write` | Ruleset create/update/delete/reset | ❌ |
| `settings.write` | System settings, GeoIP, mail (SMTP/templates/announcement), global locale writes | ❌ |
| `sso.write` | SAML/OIDC config incl. SSO role-mapping rules | ❌ |
| `sync.operate` | Sync-task list/retry/cancel/purge, reconcile-run | ❌ |
| `audit.read` | Audit log, auth-events, sub-logs, email-logs, dashboard summary, alerts | ❌ |
| `audit.clear` | Clear/purge audit, sub-logs, email-logs | ❌ |
| `roles.write` | Manage the role catalog + assignments + overrides (the RBAC surface itself) | ❌ |
| `*` (`PermAll`) | Held **only** by the immutable Global Administrator; `Has()`/subset treats it as superset of the whole catalog — future perms auto-covered, no data migration when the catalog grows. **Rejected at write time in any non-immutable role** (B2). | ❌ |

> **Note the split of node-traffic out of `traffic.read` (H3).** A node aggregates users across *all* groups; there is no `group_id` axis to filter node totals. So `/traffic/nodes/*` is gated by a **separate, non-scopeable `traffic.nodes.read`** — a group-A-scoped `traffic.read` holder cannot read cross-group node totals. Per-user traffic remains scopeable under `traffic.read`.

Pseudo-permissions (documented, never stored, never assignable): **`public`** (unauthenticated: health, version, i18n, auth handshakes, token `/sub`) and **`self`** (any authenticated principal on its own `/api/user/me/*`).

**The "management permission" predicate (single source of truth — resolves M1 & the self-service inventory).** Define once, in `domain`:
```go
// A permission that grants power beyond a pure end-user. Everything except the
// two pseudo-perms. Used by: self-service exemptions, Require2FAForStaff,
// staff-alert recipients, "land on admin console" redirect, selectIsStaff.
func (p Permission) IsManagement() bool { return p != "" /* real catalog perm */ }
func (e *EffectivePermissions) HasAnyManagement() bool // superuser || any global/scoped perm
```
"Staff/privileged" everywhere becomes `eff.HasAnyManagement()` — never the `admin||operator` literal.

**Seeded role → permission sets** (must reproduce today's access exactly, guaranteeing a zero-behavior-change migration):

- **`admin`** (Global Administrator, immutable): `["*"]`.
- **`operator`** (editable): `users.read, users.write, users.delete, traffic.read, traffic.write, sync.operate, nodes.read, nodes.toggle, groups.read, content.read, audit.read`. *Deliberately excludes* `users.elevate`, `traffic.nodes.read`, and all `*.write` admin-grade perms. This is the precise closure of today's `staffGroup` access minus the admin-only `adminGroup` routes. (Operators keep per-user traffic; node-aggregate traffic was admin-grade already — confirm against `admin_traffic.go` route grouping during Phase 2 parity tests and adjust the seed if today's operator could reach node-top.)
- **`user`** (editable): `[]`.

---

## 3. Data model

Follows `schema.go` conventions: JSON collections pinned to `text` via `GormDBDataType` (so Postgres doesn't infer `text[]`), AutoMigrate-safe (never drops columns), 3-dialect. Register new rows in `schemaModels` (`schema.go:1218`).

### 3.1 `roles` — permissions as a JSON array (not a join table)

Rationale: the codebase convention is to pin every collection to a `text` JSON column (`jsonStrings.GormDBDataType`, `schema.go:1057`); AutoMigrate emits **no FKs** anyway (`xui_panel_repo.go:113`), so a join table buys no referential integrity, only row churn. Permissions are a small bounded set always read whole — the exact shape of `EnabledRuleSets`/`Node.Tags`/`jsonRoleRules`.

```go
type roleRow struct {
    ID          int64       `gorm:"primaryKey;autoIncrement"`
    Slug        string      `gorm:"size:64;uniqueIndex;not null"`   // == domain.Role; users.role FK-by-convention
    Name        string      `gorm:"size:128;not null"`              // "Global Administrator"
    Description string      `gorm:"size:255"`
    Builtin     bool        `gorm:"not null;default:false"`         // cannot be deleted
    Immutable   bool        `gorm:"not null;default:false"`         // GA only: cannot be edited; perms locked to ["*"]
    Permissions jsonStrings `gorm:"column:permissions"`             // reuse existing text-pinned wrapper
    CreatedAt   time.Time
    UpdatedAt   time.Time
}
func (roleRow) TableName() string { return "roles" }
```

Reuse `jsonStrings` (already Value/Scan/text-pin); convert to `[]domain.Permission` in `toDomain`.

### 3.2 `role_assignments` — additional, optionally scoped grants (a table)

A table (not JSON on the user) because a user holds several, they're admin/SSO-managed, and reconcile/scope checks want indexed `WHERE user_id=` / `WHERE role_slug=`.

```go
type roleAssignmentRow struct {
    ID            int64      `gorm:"primaryKey;autoIncrement"`
    UserID        int64      `gorm:"not null;index:idx_role_assign_user"`
    RoleSlug      string     `gorm:"size:64;not null;index:idx_role_assign_role"`
    ScopeGroupIDs jsonInt64s `gorm:"column:scope_group_ids"` // empty/NULL = tenant-wide; reuse text-pinned wrapper
    Origin        string     `gorm:"size:64;not null;default:admin"` // "admin" | "sso:saml:default" | "sso:oidc:default"
    CreatedAt     time.Time
}
func (roleAssignmentRow) TableName() string { return "role_assignments" }
```

`Origin` is provenance (mirrors `SSOProvider` on `userRow:34`): SSO reconcile touches **only** its own `sso:*` rows, never an admin's hand-granted scoped assignment. `(user, role, scope)` uniqueness can't be a DB index (scope is a JSON blob) — dedupe in the service.

> **New `jsonInt64s` wrapper** (Value/Scan/text-pin) is needed for `ScopeGroupIDs` and reused for override scope. Mirror `jsonStrings` exactly (Value returns `"[]"` when nil, Scan tolerates NULL); add its own `schema_guard` reflection assertion.

### 3.3 `permission_overrides` — JSON column on `userRow` (asymmetric, deliberate)

Opposite call from assignments: overrides ride the **existing 60s live-user fetch** in `RequireAuth`, so effective-permission computation needs **zero extra hot-path query**; they're per-user, never queried cross-user. Read-modify-write of one column matches the column-scoped-writer rule.

```go
// added to userRow (schema.go:23-109), mirroring jsonRoleRules exactly:
PermissionOverrides jsonPermOverrides `gorm:"column:permission_overrides"`
```

```go
// domain:
type PermissionEffect string
const ( EffectGrant PermissionEffect = "grant"; EffectDeny PermissionEffect = "deny" )
type PermissionOverride struct {
    Permission    Permission       `json:"permission"`
    Effect        PermissionEffect `json:"effect"`
    ScopeGroupIDs []int64          `json:"scope_group_ids,omitempty"` // empty = global override
}

// sqlstore (mirror jsonRoleRules :1080-1116 — Value returns "[]" when nil, Scan tolerates NULL):
type jsonPermOverrides []domain.PermissionOverride
func (jsonPermOverrides) GormDBDataType(*gorm.DB, *schema.Field) string { return "text" }
```

Add a **column-scoped writer** `UpdatePermissionOverrides` and add `permission_overrides` to the disjoint owned-column set (same treatment as the TOTP columns, `schema.go:88-93`) so the generic `Update` never clobbers it.

### 3.4 Migration (idempotent, AutoMigrate-safe)

1. AutoMigrate adds `roles`, `role_assignments`, and the `users.permission_overrides` column (defaults `[]`/NULL; `Scan` tolerates NULL — no backfill).
2. `seedBuiltinRoles(db)` called from `EnsureSchema` after AutoMigrate:
   - **`admin` is upserted, self-healing (M4):** on every boot, force the row to `{Name:"Global Administrator", Builtin:true, Immutable:true, Permissions:["*"]}` — safe precisely because it's immutable (there are no admin edits to preserve). If a bad migration or manual DB edit ever drifts the row so it no longer holds `*`, boot repairs it, so no GA can silently lose power with no recovery role.
   - **`operator`, `user` are insert-if-not-exists by slug** (never overwrite later admin edits) — mirrors `initAdminIfNeeded` (`app.go:1269`), seeded per §2 with `Builtin:true, Immutable:false`.
3. **No `users.role` UPDATE** — we keep slug `admin`. `CountEnabledAdmins` (`user_repo.go:163`, `WHERE role='admin' AND enabled=true`) is unchanged and already means "enabled Global Administrators."
4. **First-boot `TokenVersion` bump is OFF by default (L2).** A mass bump logs out every user simultaneously (login storm on a large tenant) and merely duplicates the per-mutation bumps; the ≤60s LRU convergence the design already guarantees makes it unnecessary. Leave it as a documented, explicitly-opt-in flag for operators who want a clean reissue.

*TDD order:* `schema_guard_test` reflects over the two new rows + new column + `jsonInt64s` wrapper (cross-dialect), then a seed test asserting (a) idempotency, (b) `operator`/`user` edits survive re-boot, and (c) **the immutable `admin` row is repaired to `["*"]` if pre-seeded with a corrupted permission set.**

---

## 4. Domain & ports

### 4.1 Domain (`internal/domain/` — pure, zero deps)

- `permission.go`: `type Permission string`; the consts + `PermAll = "*"`; `var AllPermissions []Permission` (canonical catalog excl. `*`, for payload validation + SPA picker + `*` expansion); the `IsManagement()` predicate and the **`GroupScopeable()` predicate** (the ❌/✅ column of §2, used by invariant #5).
- `RoleDef` = the role entity (`Slug, Name, Description, Permissions []Permission, Builtin, Immutable bool`). `domain.Role` **stays a string slug**.
- `EffectivePermissions` — the resolved per-request snapshot, **immutable once built** (shared across concurrent requests via the LRU; build once, never mutate in place):

```go
type EffectivePermissions struct {
    superuser bool                          // holds "*"; Has/subset short-circuit true; ignores denies
    global    map[Permission]struct{}       // held tenant-wide
    scoped    map[Permission]map[int64]bool // perm -> groupIDs where additionally held (scoped assignments/overrides)
    denied    map[Permission]struct{}       // GLOBAL denies (deny beats grant; ignored if superuser)
    deniedIn  map[Permission]map[int64]bool // SCOPED denies (deny within specific groups)
}

func (e *EffectivePermissions) Has(p Permission) bool                 // held tenant-wide (global routes)
func (e *EffectivePermissions) Allows(p Permission, gid int64) bool   // held globally OR scoped-to-gid, minus denies
func (e *EffectivePermissions) IsGlobalAdmin() bool
func (e *EffectivePermissions) GlobalSet() map[Permission]struct{}    // tenant-wide perms only (for primary-role assignability)
func (e *EffectivePermissions) ScopeGroups() []int64                  // union of groups the actor has any power over
func (e *EffectivePermissions) Superset(of *EffectivePermissions) bool
```

**`Has` semantics (H4):** `Has(p)` is true iff `superuser`, OR `p ∈ global && p ∉ denied`. A *scoped* grant never satisfies `Has` — global-only routes and primary-role assignability consult `GlobalSet()` exclusively, so a scoped `users.elevate@A` can never confer a tenant-wide role.

**`Superset` semantics (H4 — the crux of scope containment), specified precisely:** `A.Superset(B)` iff `A.superuser`, OR for **every** permission `p` that `B` holds anywhere: if `B` holds `p` globally then `A` must hold `p` globally; if `B` holds `p` only in group set `G_B`, then for each `g ∈ G_B`, `A.Allows(p, g)`. Denies on `B` do not loosen the requirement (a restriction on the target doesn't make the target easier to manage). This is the single privilege order — no `Tier` field.

Resolution order (in `internal/service/rbac`): primary-role perms (expand `"*"` → `superuser`) → union each `role_assignments` role's perms into `scoped[perm][g]` for its scope groups (empty scope → `global`) → overlay `permission_overrides` (**grant** adds to `global` or `scoped` per its scope; **deny** adds to `denied`/`deniedIn` per its scope; if `superuser`, overrides ignored entirely).

### 4.2 Ports (`internal/ports/`)

- `RoleRepo`: `List / GetBySlug / Create / Update / Delete / CountUsersWithRole`.
- `RoleAssignmentRepo`: `ListByUser / ListByRole / Create / DeleteByID / ReconcileSSO(ctx, userID, origin, desired)`.
- Add both to the `Repos` aggregate (`repos.go:1357`) and construct in `sqlstore.NewRepos` (`conn.go:99`). **Because `Build()` derives `repos := dbRepos` and overrides only the YAML repos (`app.go:229-232`), the new DB repos arrive for free** — exactly the "derive from the full DB set" rule the AuthEvent-nil bug (CLAUDE.md) exists to enforce. Do not field-by-field copy.
- Overrides need no new port — column on `userRow`, reached via a new `UserRepo.UpdatePermissionOverrides` column-writer.
- **Role-reference read surface (M2).** Deleting a role must scan *four* stores: `users` (primary), `role_assignments`, `saml_settings.RoleRules`, `oidc_settings.RoleRules` (incl. new `ScopeGroupSlugs`). `rbac.Service` alone can't reach the SAML/OIDC config repos. Add `SAMLConfigRepo.Get`/`OIDCConfigRepo.Get` reads to the ports the **app layer** already holds and implement `RoleReferences(ctx, slug) ([]RoleRef, error)` as an **app-layer aggregation** (composition root wires it into the role handler), so `rbac.Service` stays free of config-repo imports and a role can never be deleted out from under a live SSO rule or scoped assignment.

### 4.3 `rbac.Service` (`internal/service/rbac` — imports only domain/ports/pkg)

Owns: `ResolvePermissions(ctx, *domain.User) (*EffectivePermissions, error)`, role CRUD, assignment/override mutation, and the authorization helpers below. **No back-edge to `user`/`traffic`**, so it needs none of the `Set*` late-binders — **no import cycle, zero new setters.**

The in-process **roles catalog is cached** with write-through invalidation on role CRUD (like `UISettings`). Per-user assignments/overrides are read live inside the 60s LRU (see §5).

Authorization helpers (generalize the three operator guards into permission/subset/scope terms, **fail-closed**):

```go
// Actor may act on a target user at all: actor ⊇ target (Superset) AND target.GroupID ∈ actor scope
// (or actor holds the needed perm globally). 503 on repo error; only ErrNotFound waves through.
func (s *Service) CanManageTarget(ctx, actor *EffectivePermissions, target *domain.User) (bool, error)

// Confer a PRIMARY role r (inherently tenant-wide): perms(r) ⊆ actor.GlobalSet() AND actor has GLOBAL users.elevate.
func (s *Service) CanAssignPrimaryRole(ctx, actor *EffectivePermissions, r domain.Role) (bool, error)

// Confer a SCOPED assignment {role r, scope G}: every perm of r is GroupScopeable AND, for each g∈G,
// actor.Allows(perm, g); AND actor controls every g∈G (g ∈ actor.ScopeGroups() or actor holds it globally).
func (s *Service) CanAssignScoped(ctx, actor *EffectivePermissions, r domain.Role, scope []int64) (bool, error)

// Grant/deny override o: o.Permission ∈ actor's effective set at o's scope; deny-REMOVAL handled separately (§5).
func (s *Service) CanGrantOverride(actor *EffectivePermissions, o domain.PermissionOverride) bool

// Edit a role DEFINITION (B1): (newPerms \ oldPerms) ⊆ actor.GlobalSet(); newPerms must not contain "*"
// unless the role is Immutable (which is never true for an editable role); actor holds GLOBAL roles.write.
func (s *Service) CanEditRoleDefinition(actor *EffectivePermissions, old, new *domain.RoleDef) (bool, error)
```

GA (`superuser`) is a superset of everyone → only a GA can manage/redact a GA.

---

## 5. Enforcement

### 5.1 Where effective permissions are computed — per-request LRU, NOT the JWT

The JWT is a bearer token; embedding perms/scope makes every privilege change invisible for up to a full access TTL (60–120 min, `jwt.go:91`) — violating instant revocation and bloating requests. Instead, extend the **existing 60s `authUserLRU`** (`auth.go:44-53,124-216`), which already re-reads the live row and re-binds `claims.Role`:

```go
type authUserSnapshot struct {
    ID, TokenVersion   int64
    UPN                string
    Role               domain.Role
    Enabled            bool
    AutoDisabledReason domain.AutoDisabledReason
    Perms              *domain.EffectivePermissions // NEW — resolved once per user per 60s
}
```

On a **cache miss**, `RequireAuth` already holds `liveUser`; it additionally calls `resolver.Resolve(ctx, liveUser)` (roles catalog in-memory; +1 query for assignments; overrides already on the row) and caches the result. On a **hit**, zero DB work — resolution runs **at most once per user per 60s**. **No new staleness class:** the ≤60s envelope already documented for role re-bind now also bounds permission convergence.

`Resolve` errors → **deny (HTTP 500), never proceed with empty perms.** An *unknown role slug* resolves to **deny-all** (still logged in) — the one safe silent path. (Closing B2 + M2 removes the realistic routes to stranding an admin-capable custom-role holder here — L3.)

### 5.2 Middleware (`internal/transport/http/middleware/`)

`RequireAuth` gains a narrow resolver interface (keeps middleware import-light, like `UserLookup`):

```go
type PermissionResolver interface {
    Resolve(ctx context.Context, u *domain.User) (*domain.EffectivePermissions, error)
}
const CtxPerms = "psp.perms"
func RequireAuth(svc *auth.Service, users UserLookup, perms PermissionResolver) gin.HandlerFunc

func RequirePermission(p domain.Permission) gin.HandlerFunc // 401 if no ctx perms; 403 if !eff.Has(p)
func EffectiveFrom(c *gin.Context) *domain.EffectivePermissions
```

`RequireRole` is **retired from the route groups.** Because different endpoints under the shared `/api/admin` prefix need different permissions, drop the group-level role gate and attach `RequirePermission(perm)` **per route**. The group-scoped axis is checked **inside the handler** once the target's `group_id` is known — the direct analog of `ensureOperatorAllowed`.

**Shared group-move guard (H1) — a single helper so no handler forgets the destination check:**
```go
// For any mutation that can change group_id, both old and new group must be in scope.
func RequireScopedForMove(eff *domain.EffectivePermissions, perm domain.Permission, oldGID, newGID int64) error {
    if !eff.Allows(perm, oldGID) || !eff.Allows(perm, newGID) { return domain.ErrForbidden }
    return nil
}
```

Reference `Update` handler:
```go
func (h *AdminUserHandler) Update(c *gin.Context) {
    target, err := h.user.Get(ctx, id)            // fail-closed on non-NotFound (503; keep existing posture)
    eff := middleware.EffectiveFrom(c)
    newGID := target.GroupID
    if body.GroupID != nil { newGID = *body.GroupID }
    if err := middleware.RequireScopedForMove(eff, domain.PermUsersWrite, target.GroupID, newGID); err != nil {
        c.JSON(403, …); return                    // H1: covers group-move confused-deputy
    }
    ok, err := h.rbac.CanManageTarget(ctx, eff, target) // Superset+scope; 503 on err (fail-closed)
    if !ok { c.JSON(403, …); return }
    if body.Role != "" {                          // PRIMARY-role change: global elevate + assignable (H4)
        ok, _ := h.rbac.CanAssignPrimaryRole(ctx, eff, body.Role)
        if !ok { c.JSON(403, …); return }
    }
    …
}
```

**List endpoints** for a scoped actor filter by `eff.ScopeGroups()` (reuse `ListByGroup`, `user_repo.go:497`). See §5.6 for the per-handler filtering audit.

### 5.3 Generalizing the existing guards (complete inventory — folds M1)

| Current guard (file:line) | Generalization |
|---|---|
| `RequireRole` group gate (`auth.go:229-251`) | Deleted; `RequirePermission(perm)` per route + route-coverage guard (§5.7) |
| `RequireAuth` role re-bind (`auth.go:95-104`) | Also resolve + attach `EffectivePermissions`; `tv` stays the generation marker |
| `ensureOperatorAllowed` (`admin_user.go:32-56`, 13 call sites) | `CanManageTarget` (Superset + scope), fail-closed preserved |
| `guardOperatorRoleAssignment` (`:75-85`) | `CanAssignPrimaryRole` (perms(r) ⊆ actor.GlobalSet() + global `users.elevate`) |
| `shouldRedactPrivilegedSecrets` (`:63-69`) | Reveal iff `actor.Superset(target)`, else blank UUID + sub URL |
| `redactInboundForRole`/`callerIsAdmin` (`admin_node.go:916-929`) | Reveal inbound secrets iff `eff.Has(nodes.write)` |
| `operatorMayView` + `Top hideStaff` + `SetUserUsage` (`admin_traffic.go`) | `eff.Allows(traffic.read/write, target.GroupID)` + Superset; list filters to in-scope/not-more-privileged; **node-agg routes → global `traffic.nodes.read`** |
| AdminAlerts admin-only filter (`admin_alerts.go:32-42`) | Per-alert probe of the permission its deep-link needs (`certs.write`/`panels.write`/`settings.write`) |
| Self-service exemptions `DisallowUser*`/`AllowUserPersonalRules` (`user_me.go`, `auth_local.go:364`, `auth_passkey.go:63`) | Exempt actors where `eff.HasAnyManagement()`; restriction applies only to pure end-users |
| **`Require2FAForStaff` (`authpolicy/authpolicy.go:61`)** — **M1** | Keys off `eff.HasAnyManagement()`, **not** `role==admin||operator`, so a powerful custom role can't bypass staff-2FA |
| **Staff-alert recipients (`mailer/mailer.go:1223`)** — **M1** | Recipient set = users whose resolved perms satisfy `audit.read` (or `HasAnyManagement()`), so custom admins still receive alerts |
| Last-admin guard `CountEnabledAdmins` (`user_repo.go:163`, service `user.go:1261/1383/2024`) | **Extended GA floor** — see §5.5 |
| AdminServers actor-UPN read; `allowSelfServiceForDisabledUser` | **Unchanged** (audit attribution / path-based; not authz) |

### 5.4 JWT / claims — unchanged shape

Keep `uid, upn, r (advisory), tv, ff`. **Add nothing.** A perm/scope hash would only help if we trusted the token's copy of perms — we don't (resolve live). `tv` remains the single revocation lever.

### 5.5 TokenVersion bump triggers + instant-revocation lever

`tv` lives in **both** access and refresh tokens (`auth.go:24,28`), so **any `tv` bump forces a full re-LOGIN.** Bumping `tv` for a role-*definition* edit would log out **every** holder (catastrophic for the universal `user` role). Therefore two tiers, **plus a mandatory instant-revocation path for scoped-admin removal (M3):**

| Mutation | `tv` bump? | LRU invalidate? | Convergence |
|---|---|---|---|
| User disabled / auto-disabled | **Yes** (`user.go:2040`) | — | ≤60s, full re-login |
| User **primary role reassigned** | **Yes** (`user.go:1398`) | — | ≤60s, full re-login |
| Password change/reset | **Yes** (`user.go:1080/1119`) | — | ≤60s, full re-login |
| Admin "revoke sessions" break-glass button (new) | **Yes** | — | ≤60s, full re-login |
| Account deleted | No — `Get`→`ErrNotFound`→401 | — | ≤60s |
| Override/assignment **grant added** | No | optional | ≤60s, seamless |
| Override/assignment **removed/deny-lifted, or scope narrowed** | No | **Yes — default-ON (M3)** | **next request**, seamless |
| A role *definition*'s permission set edited (all holders) | No | `Clear()` on catalog CRUD | next request / ≤60s |

**Rationale + M3 fix:** grants can afford ≤60s. But *revocation of a compromised or malicious scoped admin* must not leave 60s of continued cross-scope access. So `authUserLRU.Invalidate(userID)` on any **removal/narrowing** of an assignment or override is **default-ON** (it's cheap, next-request effect), wired via a narrow `middleware.CacheInvalidator` into the mutators. Emergency response additionally bumps `tv` via the "revoke sessions" button. `authUserLRU.Clear()` fires on role-definition CRUD for instant fan-out. The old "everything opt-in/deferred" stance is dropped for removals.

### 5.6 Per-handler scope-filtering audit (folds missing-item #8)

Every list/read endpoint tagged scopeable gets an explicit filter spec; unscopeable-in-principle families are pinned to global perms:

| Handler | Filter for a scoped actor |
|---|---|
| `GET /users`, `/users/:id/*` | `ListByGroup(eff.ScopeGroups())`; single-get 404/403 if `target.GroupID ∉ scope` |
| `GET /groups` | Filter to `eff.ScopeGroups()` (plus any group where actor holds `groups.read` globally) |
| `GET /nodes*` | **Not group-addressable** → `nodes.read` is global-only; scoped actors don't see nodes unless they hold it globally |
| `GET /traffic/user/:id`, `/history`, `/top` | Per-user rows filtered to in-scope users; `Top` restricted to in-scope user set |
| `GET /traffic/nodes/top`, `/nodes/history` | **`traffic.nodes.read` (global-only, H3)** — never reachable by a purely scoped actor |
| `GET /alerts` | Per-alert probe of the deep-link permission (global perms only) |
| `GET /audit`, `/auth-events`, `/sub-logs`, `/email-logs`, `/dashboard/summary` | `audit.read` is **global-only** in v1 (cross-group aggregates; no reliable `group_id` axis) — a scoped actor without global `audit.read` gets 403, not a partial view. (Scoped audit is deferred; documented non-goal.) |

### 5.7 Route-coverage completeness guard (folds B3)

Retiring the group gate means any un-tagged `/api/admin` route would become authenticated-open (reachable by *any* logged-in user). A "role×route matrix unchanged" test cannot catch a *forgotten* route — it's simply absent from the matrix.

**Mechanical guarantee (lands WITH Phase 2, before Phase 4 removes the net):**
```go
// boot-time assertion + a unit test over the same function
func assertEveryAdminRouteGated(r *gin.Engine) error {
    allow := map[string]bool{ /* explicit public/self exceptions under /api/admin, if any */ }
    for _, ri := range r.Routes() {
        if !strings.HasPrefix(ri.Path, "/api/admin") || allow[ri.Method+" "+ri.Path] { continue }
        if !handlerChainContains(ri, "RequirePermission") { // tag each RequirePermission closure for detection
            return fmt.Errorf("%w: ungated admin route %s %s", domain.ErrValidation, ri.Method, ri.Path)
        }
    }
    return nil
}
```
`Build()` calls it after route registration and **refuses to boot** on a gap; a test enumerates `router.Routes()` and fails identically. Detection: wrap each permission gate so its handler carries a recognizable marker (e.g. a named func or a registry keyed by handler pointer).

---

## 6. Route → permission map

Every route carries one permission tag; scopeable families additionally run the per-handler scope/subset guard (§5.6). Public/self families keep no permission gate. **The §5.7 completeness guard is the backstop for anything omitted here.**

| Route family (examples) | Group today | Permission | Per-handler scope guard |
|---|---|---|---|
| `GET /users`, `/users/:id`, `/rules`, `/passkeys` | staff | `users.read` | filter/`CanManageTarget` on `target.GroupID` |
| `POST /users`, `PUT /users/:id`, resets, `set-enabled`, `set-service-status`, `unlink-sso`, `PUT rules`, passkey revoke | staff | `users.write` | `RequireScopedForMove` (old+new group) + `CanManageTarget` |
| `DELETE /users/:id` | staff | `users.delete` | + `CanManageTarget` |
| Primary-role change on create/update | staff | `users.write` **+ global `users.elevate`** | + `CanAssignPrimaryRole` |
| `GET /nodes`, `/nodes/:id`, `/unmanaged`, `/separator` | staff | `nodes.read` (global) | — (secrets redacted unless `nodes.write`) |
| `POST /nodes/:id/set-enabled` | staff | **`nodes.toggle`** | — |
| all other node/inbound/reality/import/detach/recreate/claim; `cert-source` | admin | `nodes.write` | claim also needs `users.write` over linked user's group |
| `GET /groups`, `/groups/:id` | staff | `groups.read` | filter to scope |
| `POST /groups` | admin | `groups.write` | (create inherently global) |
| `PUT /groups/:id`, `/layout`, `/scope-settings*`, `DELETE /groups/:id` | admin | `groups.write` | scopeable by `:id`; **TagFilter edits stay global-only** (cross-group blast radius); refuse deleting a referenced group |
| `GET /rules`, `/templates`, `/locales` | staff | `content.read` | — |
| `PUT/DELETE/reset /rules/:slug` | admin | `rulesets.write` | — |
| `PUT/DELETE/reset /templates/:slug` | admin | `templates.write` | — |
| `PUT/DELETE /locales/:code` | admin | `settings.write` | — |
| all `/servers*` | admin | `panels.write` | — |
| all `/certs*`, `/dns-*`, `/acme-*`, `cert-events` | admin | `certs.write` | download stays write-grade (private keys) |
| `GET/PUT /settings/ui`, `geoip*`, `mail*` | admin | `settings.write` | announcement stays global (mass-email blast) |
| `GET/PUT /settings/saml`, `/settings/oidc`, `saml/fetch` | admin | `sso.write` | rule-role validated against catalog (§7.1) |
| `traffic` per-user read: `top`, `history`, `user/:id/*` | staff | `traffic.read` | `Allows(traffic.read, target.GroupID)`; `Top`/list filter to in-scope |
| **`traffic` node-aggregate: `nodes/top`, `nodes/history`** | staff | **`traffic.nodes.read` (global-only, H3)** | — (never scoped) |
| `POST /traffic/poll` | staff | `traffic.write` | global (no target) |
| `PUT /traffic/user/:id` | staff | `traffic.write` | `Allows` + Superset |
| `sync-tasks` list/retry/cancel/purge, `reconcile/run` | staff/admin | `sync.operate` | — |
| `audit`, `auth-events`, `dashboard/summary`, `alerts`, `sub-logs` GET, `email-logs` GET | staff | `audit.read` (global-only, §5.6) | alerts filtered per deep-link perm |
| `DELETE audit`, sub-logs/email-logs clear/purge | admin | `audit.clear` | — |
| **NEW** `/api/admin/roles*`, `/rbac/permissions`, `/users/:id/assignments*`, `/users/:id/overrides` | admin | `roles.write` | writes run grant-only-what-you-hold; **self-target forbidden (§8)** |
| `/api/user/me/*` | user | `self` | — |
| health, version, i18n, auth handshakes, `/sub/:token`, SPA | public | `public` | — |

---

## 7. SSO integration

### 7.1 Validate rule role against the catalog

Keep `parseRoleString` (`role.go:151`) free-form at login (don't hard-fail mid-login). **Validate at rule-save time** in `admin_saml.go` / `admin_oidc.go`: every `SSORoleRule.Role` must resolve to an existing role slug, else `ErrValidation` (`%w`). At login, a matched-but-since-deleted slug → fall back to `RoleUser` + warn (never fail login). `RoleRulesEditor.builtinRoleSuggestions` (`SettingsView.tsx:1858`) becomes a **live dropdown fed by `GET /api/admin/roles`.**

### 7.2 Optional scope on rules

Extend `config.SSORoleRule` (`config/sso.go:34`) with `ScopeGroupSlugs []string` (`yaml/json`). The existing `jsonRoleRules` column deserializes old rules with the field defaulting empty — fully backward-compatible.

- **Empty scope** → rule sets the **primary role** (`users.role`), unscoped (today's behavior; `MatchFirstRule`/`ResolveRoleForSSO` Keep-on-miss matrix unchanged, now over only the unscoped subset).
- **Non-empty scope** → rule contributes a **scoped `role_assignments`** row (`Origin=sso:<provider>`); primary role untouched by this rule.

Add `MatchScopedAssignments(rules, attrs, groups) []DesiredAssignment` (returns **all** matches, not first-match-wins). The empty-`Value` guard (`role.go:116-118`) stays load-bearing. **A scoped SSO rule whose role carries a global-only permission is rejected at rule-save time** (same invariant #5 gate as admin-created scoped assignments).

### 7.3 EnsureSSO / reconcile (`user.go:751-919`)

Primary-role path unchanged. **After** the row exists, call `roleAssignments.ReconcileSSO(ctx, userID, "sso:<provider>", desired)` — authoritative **only over its own `Origin='sso:*'` rows**: add missing, drop rules the IdP no longer grants (honor per-rule `Keep`), **never touch `Origin='admin'` rows**. Because SSO reconcile changes assignments (not the primary role), it uses **LRU re-resolve; removals trigger the default-ON `Invalidate` (M3)**, no `tv` bump; primary-role changes keep their existing `tv` bump.

### 7.4 `privilegedRuleMatch` generalization (`user.go:710-716`)

Replace the hardcoded `role == RoleAdmin || RoleOperator` with **"the matched rule grants a role holding any permission beyond baseline user"** (primary or scoped):

```go
func privilegedRuleMatch(ctx, in, roles ports.RoleRepo) bool {
    if r, ok := auth.MatchFirstRule(...); ok && roleHasElevatedPerms(ctx, roles, r) { return true }
    for _, d := range auth.MatchScopedAssignments(...) {
        if roleHasElevatedPerms(ctx, roles, domain.Role(d.RoleSlug)) { return true }
    }
    return false
}
// roleHasElevatedPerms: role exists AND (holds "*" OR len(perms) > 0)
```

`user.Service` gains a `roles ports.RoleRepo` dependency (it already holds repos). Preserves bootstrap semantics (IdP affirmatively elevated → trust enough to create despite `AllowAutoCreate` off).

### 7.5 GA guard for SSO

**SSO must not grant the immutable primary `admin` role** without an explicit **default-off** setting (`AllowSSOGlobalAdmin`). A compromised/misconfigured IdP group→role rule is otherwise full takeover. Even if enabled, immutability + the last-GA floor stay authoritative. Because `*` is forbidden in every editable role (B2), SSO also cannot route around this by granting a custom `*`-bearing role — no such role can exist.

---

## 8. API surface

New endpoints under `adminGroup`, gated `roles.write`. Every write runs **grant-only-what-you-hold** (§11) and **forbids self-targeting.**

```
GET    /api/admin/roles                       list roles (builtin/immutable/permissions)
POST   /api/admin/roles                       create custom role  — CanEditRoleDefinition(∅→new); reject "*"
PUT    /api/admin/roles/:slug                 edit — 409 if immutable; CanEditRoleDefinition(old→new); reject "*"
DELETE /api/admin/roles/:slug                 delete — 409 if builtin OR RoleReferences(slug) non-empty (users+assignments+SAML+OIDC)
GET    /api/admin/rbac/permissions            catalog + i18n descriptions + scopeable flag (SPA picker)

GET    /api/admin/users/:id/assignments       list scoped assignments
POST   /api/admin/users/:id/assignments       {role_slug, scope_group_ids} — CanAssignScoped; reject if :id == actor
DELETE /api/admin/users/:id/assignments/:aid  reject if :id == actor; triggers Invalidate(:id)
PUT    /api/admin/users/:id/overrides         replace overrides — CanGrantOverride per added grant;
                                              deny-removal privileged (§ below); reject if :id == actor
```

**Self-target policy (folds missing-item #6, H2).** The three RBAC write endpoints (`/roles/*` self-assign paths, `/assignments`, `/overrides`) and primary-role change **reject `path user_id == actor user_id` outright** (`%w ErrForbidden`) — closing the "edit a role I hold, then it applies to me" and "lift my own deny" self-escalation surfaces. Only a GA may alter another GA; a GA altering itself is likewise blocked from *reducing* the GA floor by the last-GA guard.

**Deny-removal semantics (H2).** `PUT /overrides` is replace-semantics, so the handler **diffs old vs new**:
- *Added grant* → `CanGrantOverride` (perm ∈ actor at that scope).
- *Removed/loosened `deny`* → **privileged**: requires `actor.Superset(target)` (strict, minus the deny being removed) AND target ≠ actor (self-removal of a deny is forbidden entirely). A `roles.write` holder can therefore never lift a GA-imposed deny on themselves or on a peer they don't strictly dominate.
- *Unchanged entries* → no check.

Primary-role change stays on `PUT /api/admin/users/:id`, gated global `users.elevate` + `CanAssignPrimaryRole`.

**Session payload (source of truth).** Extend `userBrief` (`auth_local.go:194`) — shared by login, 2FA-verify, passkey-login-finish, sso-complete, and `/api/user/me`:

```go
type userBrief struct {
    ID int64; UPN string; DisplayName string
    Role        domain.Role  `json:"role"`         // primary role slug (kept, for display)
    RoleLabel   string       `json:"role_label"`   // role display name for the account-menu chip
    Permissions []Permission `json:"permissions"`  // resolved GLOBAL set ("*"→full expanded catalog)
    ScopedPermissions []struct{ Permission Permission `json:"permission"`; GroupIDs []int64 `json:"group_ids"` } `json:"scoped_permissions,omitempty"`
    ScopeGroups []int64 `json:"scope_groups,omitempty"`
}
```

**Refresh gap:** the axios single-flight 401 refresh (`client.ts`) does not re-fetch perms — mitigated because primary-role/disable/password bumps `tv` (→ forced re-login), per-user removals trigger default-ON `Invalidate` (next request), and all other edits converge ≤60s. Optional: a lightweight `GET /api/user/me/permissions` the SPA polls to refresh without re-login.

---

## 9. Frontend

`web-react/src/utils/permissions.ts` — its header comment already predicts this move:

- **Delete `ROLE_CAPS`** (29-33). Change the `Capability` union (14-27) into the fine-grained `Permission` string set. Rewrite `roleCan`→`permissionSetHas`; **`useCan` reads `s.permissions`** instead of `roleCan(s.role, cap)`. **Every existing call site's mapped key changes but the call shape stays:**
  - GroupsView `config.write`→`groups.write`; NodesView `config.write`→`nodes.write` (toggle button → `nodes.toggle`); TemplatesView→`templates.write`; RuleSetsView→`rulesets.write`; LanguagePacksView→`settings.write`.
  - LogsView **splits**: view/export → `audit.read`; clear-all (422) → `audit.clear`.
  - SyncTasksView purge/retry/cancel → `sync.operate` (**add** the currently-missing gate on batch retry/cancel, 233/238).
  - UsersView: `canElevate`→`users.elevate`; role `<Select>` `disabled={auth.role==='operator'}` (1566/1567) → `!useCan('roles.write')`.
- **Add a scoped variant** `useCanForGroup(perm, groupId)` consulting the scope descriptor — used by `UsersView.canManageUser` (237) and per-group node/user actions.
- **Per-button gates for the newly-split admin actions (L1)** — explicitly enumerate so none render-then-403 (backend enforces regardless, this is cosmetic): cert download/renew → `certs.write`; server probe/upgrade → `panels.write`; SSO tab within Settings → `sso.write` (separate from `settings.write`); role-manager edit/delete → `roles.write`.

Auth store (`auth.ts`):
- Add `permissions: Permission[]` (store as a `Set` for O(1) `useCan`) + a scope descriptor to **both** `AuthState` (26) and `PersistedAuthState` (55). `role` kept for display.
- Wire into `applySession` (72-83) and `loginSSO` (124-136 — refactor to call `applySession` so permission plumbing lives once); include in `loadFromStorage`/persist/`setDisplayName` (58-68, 138-143).
- Reimplement selectors off perms: `selectIsStaff`→`hasAnyManagement(perms)`; `selectIsAdmin` has no single equivalent — its call sites (`RequireAuth:30`, `LoginView:152`) switch to per-path/management-permission checks.

Routing (`home.ts`/`RequireAuth.tsx`/`AdminLayout.tsx`): introduce **one `path → required-permission` table** replacing `ADMIN_ONLY_ROUTES` and per-`NavItem` `adminOnly` booleans, so route guards, post-login redirect, and sidebar gate off the same map (`/admin/servers`→`panels.write`, `/admin/settings`→`settings.write` (+`sso.write` for the SSO tab), certs→`certs.write`, language-packs→`settings.write`). `homeForRole`→"land on admin console iff `hasAnyManagement`, else `/user/me`."

Types (`types.ts`): `Role` → `string` (custom slugs); `AuthLoginResponse.user` gains `permissions`, scope fields, `role_label`.

New admin UI:
- **Roles manager** (list/create/edit/delete; permission-picker checkbox grid from `GET /rbac/permissions`; immutable/builtin badges disable edit/delete; **`*` never selectable — it isn't in the catalog list**).
- **Per-user assignment + override editor** in `UsersView` drawer (scoped role assignments with a group multi-select; grant/deny override list; **self-row shows read-only** since self-targeting is server-forbidden).
- **RoleRulesEditor** (`SettingsView.tsx:1851`): role dropdown fed by `GET /roles`; optional scope-group multi-select per rule.
- Role display everywhere (`AdminLayout:456`, `UsersView` badges/MenuItems/help 985-1613, `AccountSecurityDrawer:122`) becomes **data-driven from the roles list**, showing `role_label`.

i18n: permission display names/descriptions + role-management UI strings in **both** `src/locales/zh-CN` (base) and `en-US`.

---

## 10. Phased rollout (each independently shippable, TDD, never lock-out)

Belt-and-suspenders: **`RequireRole` stays live until `RequirePermission` is proven**, seeded roles reproduce today's access exactly, and the **route-coverage guard (§5.7) lands with Phase 2 — before Phase 4 removes the group gate.** The extended GA floor lands before any role-unassign/scope mutation is exposed.

- **Phase 0 — Catalog + schema (no enforcement change).** `domain.Permission` + catalog + `IsManagement`/`GroupScopeable`; `roles`/`role_assignments` rows + `permission_overrides` column + `jsonInt64s`; seed built-ins with **self-healing immutable `admin` (M4)**; keep slug `admin`. *Failing tests first:* `schema_guard` cross-dialect over new rows/column/wrapper; idempotent seed; operator/user edits survive reboot; **corrupted `admin` row repaired to `["*"]`**.
- **Phase 1 — `rbac.Service` + resolution (shadow).** `ResolvePermissions` truth table (superuser / global vs scoped deny / scope / unknown-slug→deny-all). Wire resolver into `RequireAuth` populating `CtxPerms`, **not yet enforcing**. *Tests:* resolution truth table incl. scoped-deny, fail-closed on resolve error, `Superset` over the scoped map.
- **Phase 2 — `RequirePermission` alongside `RequireRole` (both must pass) + route-coverage guard.** Map every route (§6); **§5.7 completeness assertion at boot + test**. No behavior change (seeded roles = current powers). *Tests:* route-permission parity; each seeded role reproduces its exact pre-refactor access matrix; **no ungated `/api/admin` route**.
- **Phase 3 — Replace per-handler operator guards.** `CanManageTarget`/`CanAssignPrimaryRole`/redaction/`nodes.write`-keyed reveal/traffic filters/`RequireScopedForMove`; **fail-closed preserved**; **M1 redefinitions (`Require2FAForStaff`, staff-alert recipients)**. *Tests:* escalation blocked, fail-closed on DB error, Superset relation, secret redaction, staff-2FA can't be bypassed by a custom role.
- **Phase 4 — Retire `RequireRole` group gate.** Rely solely on `RequirePermission` + handler scope checks (route-coverage guard is the backstop). *Tests:* full role×route matrix unchanged from Phase 2; coverage guard still green.
- **Phase 5 — Scoped assignments + overrides + role CRUD API.** Grant-only-what-you-hold on **assignment AND role-definition edit (B1)**; `*`-rejection in editable roles (B2); deny-precedence + **deny-removal guard (H2)**; self-target rejection (§8); `RoleReferences` delete guard (M2); **extended last-GA floor across delete/disable/demote/scope-add** lands here; default-ON `Invalidate` on removal (M3). *Tests:* scope enforcement, deny>grant, `CanEditRoleDefinition` blocks add-perm-you-lack, `*` rejected in custom role, deny-removal blocked for non-superset/self, last-GA lockout on all flows, scoped-assignment global-perm rejection, delete-referenced-role blocked, self-target rejected.
- **Phase 6 — SSO scoped assignments.** Rule validation against catalog, optional scope, `ReconcileSSO` origin-only + `Keep`, `privilegedRuleMatch` generalization, `AllowSSOGlobalAdmin` default-off, scoped-SSO-rule global-perm rejection. *Tests:* reconcile touches only `sso:*` rows, Keep honored, GA-grant blocked without toggle, empty-Value guard, deleted-slug→user+warn.
- **Phase 7 — Frontend.** Session payload, `permissions.ts` rewrite (call sites stable), path→perm table, roles manager + per-user editor + RoleRulesEditor, per-button gates (L1), data-driven role labels, i18n (zh-CN base + en-US). *Tests:* build (frontend before backend for `go:embed`), gate-parity smoke.

---

## 11. Security invariants (each with the test that proves it)

1. **Last Global Administrator can never be locked out.** No delete / disable / demote-away-from-`admin` / **adding a group-scope constraint** may drop enabled-unscoped-`admin` count below 1; the immutable `admin` role cannot be edited/deleted; a `deny` override on a GA is ignored; the row is **self-healed to `["*"]` on boot**. *Test:* each flow on the sole GA → rejected (`%w ErrForbidden`/`ErrValidation`); `Has` still true for a GA carrying a `deny`; boot repairs a corrupted `admin` row.
2. **Grant-only-what-you-hold — assignment path.** No role assignment / override-grant / primary-role change may confer a permission the assigner's own effective set lacks (primary-role uses `GlobalSet()`). *Test:* actor with `roles.write` but not `panels.write` self-assigns a role containing `panels.write` → 403; override-grant of `panels.write` → 403.
3. **Grant-only-what-you-hold — role-definition edit (B1).** `CanEditRoleDefinition` requires `(newPerms \ oldPerms) ⊆ actor.GlobalSet()`. *Test:* `roles.write` holder lacking `settings.write` edits a role to add `settings.write` → 403 on both Create and Update.
4. **`*` forbidden in editable roles (B2).** Any Create/Update whose permission set contains `*` on a non-immutable role → `%w ErrValidation`. *Test:* create custom role `{"*"}` → 400; the only `*`-bearer remains the immutable `admin`, which `CountEnabledAdmins` sees.
5. **Every `/api/admin` route is permission-gated (B3).** Boot + test enumerate `router.Routes()` and fail on any un-tagged admin path. *Test:* register a deliberately un-gated admin route in a fixture → assertion fails.
6. **No acting on a peer or superior.** `CanManageTarget` requires `actor.Superset(target)`; only a GA may manage a GA. *Test:* operator-equivalent A cannot edit/reset/delete operator-equivalent B; only GA edits GA.
7. **Scope confined; no confused-deputy incl. group-move (H1).** A group-A-scoped manager cannot read/mutate an out-of-scope user, **move a target into or out of scope** (`RequireScopedForMove` checks both old and new group), or create an assignment covering a group it doesn't control. *Test:* scoped actor `PUT` setting `group_id=B` on an A-user → 403; assignment with uncontrolled scope → 403.
8. **Scoped grants carry only group-addressable permissions.** Reject any scoped assignment/override/SSO-rule carrying a `GroupScopeable()==false` perm at write time. *Test:* `POST assignments {role: X-with-nodes.write, scope:[A]}` → 400; same for a scoped SSO rule.
9. **Scoped subset semantics (H4).** Primary-role assignability uses `GlobalSet()` only (a scoped `users.elevate@A` never confers a tenant-wide role); scoped assignment over G requires each perm held globally or scoped-to-each-`g∈G`. *Test:* scoped-only actor cannot assign any primary role; scoped `CanAssignScoped` truth table over the scoped axis.
10. **Deny beats grant beats role — except superuser; deny-removal is privileged (H2).** *Test:* `{role:user + grant users.read + deny users.read}` → no `users.read`; GA + any deny → still full; a `roles.write` holder cannot PUT-away a GA-imposed deny on self or on a non-dominated peer.
11. **Fail-closed end-to-end.** `RequireAuth` denies (500) on `Resolve` error; guards 503 on non-`ErrNotFound` lookup error; unknown role slug → deny-all (not error). *Test:* inject repo error at each site → deny, never empty-perm proceed.
12. **Self-target forbidden (missing-item #6).** Assignment/override/primary-role/role-self-assign APIs with `path_uid == actor_uid` → `%w ErrForbidden`. *Test:* actor calls each self-targeted endpoint → 403.
13. **Stale-session bound ≤60s; removals revoke by next request; no mass re-login on role-def edits (M3).** Primary-role/disable/password bump `tv`; assignment/override **removal** default-ON `Invalidate` (next request); role-definition edits `Clear()` (next request). *Test:* narrowing a scoped admin's scope removes access on the immediately-following request without a `tv` bump; editing the `user` role's perms doesn't log anyone out.
14. **SSO cannot silently mint a Global Administrator.** With `AllowSSOGlobalAdmin=false`, a rule resolving to primary `admin` → `user` + warn; reconcile never touches `Origin='admin'` rows; empty-`Value` never matches; no `*`-bearing custom role can exist (B2) to route around it. *Test:* IdP asserts GA group, toggle off → provisioned as `user`; hand-granted scoped assignment survives an SSO reconcile only when `Keep` set.
15. **Role deletion is referentially safe (M2).** `DELETE /roles/:slug` blocked if the slug is any primary role, any assignment, or any SAML/OIDC rule (`RoleReferences` aggregation). *Test:* delete a role referenced only by an OIDC rule → 409.
16. **`Require2FAForStaff` / staff alerts follow permissions, not literals (M1).** *Test:* a custom management role triggers staff-2FA enforcement and appears in alert recipients; a bare `user` does neither.
17. **`EffectivePermissions` immutable & race-safe.** Built once on cache-miss, never mutated in place. *Test:* concurrent readers of one user's snapshot observe a fully-populated set (`-race`).

### 11.1 Security review resolutions

| Finding | Resolution in this plan |
|---|---|
| **B1** — role-def edit unguarded (self-escalation) | Added `CanEditRoleDefinition(actor, old, new)` requiring `(newPerms\oldPerms) ⊆ actor.GlobalSet()`, enforced on **every** role Create *and* Update (§4.3, §8), Phase-5 invariant #3 with its own test. |
| **B2** — `*` allowed in custom roles (shadow GA) | `*` rejected (`%w ErrValidation`) in any non-immutable role at write time (§2, §8); catalog/UI never surface it; invariant #4. Removes the lockout blind spot and the B1×`*` escalation vector. |
| **B3** — retiring `RequireRole` opens un-tagged routes | Mechanical **route-coverage guard** enumerates `router.Routes()`, refuses boot + fails a test on any un-gated `/api/admin` path; **lands with Phase 2, before Phase 4** (§5.7, invariant #5). |
| **H1** — group-move confused-deputy | Shared `RequireScopedForMove(old,new)` helper checks **both** source and destination group on any `group_id`-changing mutation; baked into the reference handler (§5.2, invariant #7). |
| **H2** — deny-removal unguarded | Override PUT **diffs** old vs new; removing/loosening a `deny` is privileged (requires strict `Superset`, self-removal forbidden); self-target rejected outright (§8, invariant #10 & #12). |
| **H3** — node-traffic leaks cross-group | Node-aggregate traffic split into a **separate global-only `traffic.nodes.read`**; scopeable `traffic.read` covers per-user only (§2, §5.6, §6). |
| **H4** — scoped `Superset`/assign semantics undefined | `Has`/`GlobalSet`/`Superset` specified over the scoped map; primary-role assignability uses **global** set + global `users.elevate`; `CanAssignScoped` requires per-group holding (§4.1, §4.3, invariant #9). |
| **M1** — `Require2FAForStaff` & staff-alert recipients missed | Both re-routed through `HasAnyManagement()`; added to the §5.3 inventory; invariant #16. |
| **M2** — role-delete referential check incomplete | App-layer `RoleReferences(slug)` aggregation spans users + assignments + SAML + OIDC (via read ports the app layer holds); `rbac.Service` stays config-repo-free (§4.2, §8, invariant #15). |
| **M3** — no instant revocation for scoped-admin removal | `Invalidate(userID)` on any assignment/override **removal/narrowing** is **default-ON** (next-request); emergency "revoke sessions" bumps `tv` (§5.5, invariant #13). |
| **M4** — seed doesn't self-heal GA row | `admin` row **upserted every boot** to `{Immutable, ["*"]}` (safe because immutable); seed test asserts repair of a corrupted row (§3.4, invariant #1). |
| **L1** — per-button gates for split perms | Enumerated explicitly for `certs.write`/`panels.write`/`sso.write` (§9). |
| **L2** — mass `tv` bump on first boot | Off by default; rely on ≤60s LRU convergence (§3.4). |
| **L3** — unknown-slug stranding | Structurally removed by closing B2 + M2 (§5.1). |
| **#6** — self-target policy unstated | Self-targeted RBAC writes forbidden (`ErrForbidden`); invariant #12, §8. |
| **#8** — per-handler scope-filtering audit | Concrete filter spec table for every list/read handler; `audit.*` and `nodes.read` pinned global-only (§5.6). |
| **#9** — `ErrValidation` vs `ErrForbidden` mixing | Fixed classification convention stated up front (payload-invalid → `ErrValidation`; actor-forbidden → `ErrForbidden`; infra → 503). |

**Files that change (anchors):** `internal/domain/{permission.go(new),enums.go,types.go}`, `internal/ports/{repos.go,saml_config_repo.go,oidc_config_repo.go}`, `internal/adapters/sqlstore/{schema.go,role_repo.go(new),role_assignment_repo.go(new),user_repo.go,conn.go}`, `internal/service/rbac/*(new)`, `internal/service/auth/role.go`, `internal/service/authpolicy/authpolicy.go`, `internal/service/mailer/mailer.go`, `internal/service/user/user.go`, `internal/transport/http/middleware/auth.go`, `internal/transport/http/router.go`, `internal/transport/http/handler/{admin_user.go,admin_node.go,admin_traffic.go,admin_alerts.go,admin_role.go(new),auth_local.go,user_me.go,admin_saml.go,admin_oidc.go}`, `internal/app/app.go`, `internal/config/sso.go`, `web-react/src/{utils/permissions.ts,stores/auth.ts,api/types.ts,router/*,layouts/AdminLayout.tsx,views/admin/*}`, plus `src/locales/{zh-CN,en-US}/*`.