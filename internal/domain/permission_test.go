package domain

import "testing"

// The permission catalog is the contract the whole RBAC model is built on:
// route gates, seeded roles, the SPA picker and the "reproduce today's access"
// migration guarantee all index into it. Lock its shape so an accidental
// rename/drop can't silently widen or narrow access.

func TestAllPermissionsCatalog(t *testing.T) {
	want := map[Permission]bool{
		PermUsersRead: true, PermUsersWrite: true, PermUsersDelete: true, PermUsersElevate: true,
		PermTrafficRead: true, PermTrafficWrite: true, PermTrafficNodesRead: true,
		PermGroupsRead: true, PermGroupsWrite: true,
		PermNodesRead: true, PermNodesToggle: true, PermNodesWrite: true,
		PermPanelsWrite: true, PermCertsWrite: true,
		PermContentRead: true, PermTemplatesWrite: true, PermRulesetsWrite: true,
		PermSettingsWrite: true, PermSSOWrite: true,
		PermSyncOperate: true, PermAuditRead: true, PermAuditClear: true,
		PermRolesWrite: true,
	}
	if len(AllPermissions) != len(want) {
		t.Fatalf("AllPermissions has %d entries, want %d", len(AllPermissions), len(want))
	}
	seen := map[Permission]bool{}
	for _, p := range AllPermissions {
		if p == PermAll {
			t.Errorf("AllPermissions must NOT contain the wildcard %q — it is GA-only and never assignable", PermAll)
		}
		if seen[p] {
			t.Errorf("AllPermissions contains duplicate %q", p)
		}
		seen[p] = true
		if !want[p] {
			t.Errorf("AllPermissions contains unexpected permission %q", p)
		}
	}
	for p := range want {
		if !seen[p] {
			t.Errorf("AllPermissions is missing expected permission %q", p)
		}
	}
}

func TestPermissionGroupScopeable(t *testing.T) {
	// Exactly these carry a target group_id axis (Entra "administrative unit"
	// scoping). Everything else is tenant-wide only — a scoped assignment
	// carrying a non-scopeable perm is rejected at write time (invariant #8).
	scopeable := map[Permission]bool{
		PermUsersRead: true, PermUsersWrite: true, PermUsersDelete: true, PermUsersElevate: true,
		PermTrafficRead: true, PermTrafficWrite: true,
		PermGroupsRead: true, PermGroupsWrite: true,
	}
	for _, p := range AllPermissions {
		if got := p.GroupScopeable(); got != scopeable[p] {
			t.Errorf("%q.GroupScopeable() = %v, want %v", p, got, scopeable[p])
		}
	}
	// The wildcard is never group-scopeable.
	if PermAll.GroupScopeable() {
		t.Errorf("PermAll must not be group-scopeable")
	}
	// traffic.nodes.read is deliberately global-only (H3): a node aggregates
	// users across all groups, so there is no group_id axis to filter on.
	if PermTrafficNodesRead.GroupScopeable() {
		t.Errorf("traffic.nodes.read must be global-only")
	}
}

func TestPermissionIsManagement(t *testing.T) {
	// Every real catalog permission grants power beyond a pure end-user, so it
	// counts as "management" (drives staff-2FA, staff alerts, admin-console
	// landing). The empty string (absence of a permission) does not.
	for _, p := range AllPermissions {
		if !p.IsManagement() {
			t.Errorf("%q.IsManagement() = false, want true", p)
		}
	}
	if !PermAll.IsManagement() {
		t.Errorf("PermAll.IsManagement() = false, want true (GA is management)")
	}
	if Permission("").IsManagement() {
		t.Errorf("empty Permission must not be management")
	}
}
