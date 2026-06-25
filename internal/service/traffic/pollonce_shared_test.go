package traffic

import (
	"context"
	"testing"
	"time"

	"github.com/KazuhaHub/passwall-sub-panel/internal/domain"
	"github.com/KazuhaHub/passwall-sub-panel/internal/ports"
)

// suspendCapture records SetServiceSuspendedAndSync so a PollOnce quota test can
// assert the migrated user's service was suspended (and why).
type suspendCapture struct {
	fakeDisabler
	suspendedUser   int64
	suspendedReason domain.AutoDisabledReason
}

func (d *suspendCapture) SetServiceSuspendedAndSync(_ context.Context, userID int64, reason domain.AutoDisabledReason, _ string) error {
	d.suspendedUser = userID
	d.suspendedReason = reason
	return nil
}

// PollOnce Phase 2b (shared-client metering) is quota-critical and was previously
// only unit-tested via recordSharedClientStats. This drives the WHOLE poll for a
// fully-migrated user (no ownership rows) and pins two properties:
//
//  1. Single-count across inbounds: a shared client attached to N inbounds echoes
//     the SAME aggregate counter on each, so the poll must fold it ONCE (max across
//     echoes), never sum — summing would double the user's metered traffic.
//  2. Quota enforcement reaches migrated users: crossing the limit suspends their
//     SERVICE (DisabledTrafficExceeded), even with no legacy per-node hits — the
//     `tot.hits++` in Phase 2b is what makes recordAndEnforceWith run for them.
func TestPollOnce_SharedClient_SingleCountAndQuotaSuspend(t *testing.T) {
	users := &fakeUserRepo{users: map[int64]*domain.User{
		1: {ID: 1, Enabled: true, TrafficLimitBytes: 1000},
	}}
	// Migrated user: NO ownership rows. The shared client is pre-seeded with a raw
	// baseline (LastRaw != 0) so this single poll produces a real delta instead of
	// the spike-proof first-observation seed.
	psp := &fakePSPClientRepo{byUser: map[int64][]*domain.PSPClient{
		1: {{ID: 1, UserID: 1, PanelID: 10, Email: "u1@psp.local",
			LastRawUpBytes: 100, LastRawDownBytes: 100, LastRawTotalBytes: 200}},
	}}
	// Same aggregate (700/500) echoed on TWO inbounds — the multi-inbound case.
	echo := []ports.ClientTraffic{{Email: "u1@psp.local", Up: 700, Down: 500}}
	pool := &fakeXUIPool{clients: map[int64]ports.XUIClient{
		10: &fakeXUIClient{inbounds: []ports.Inbound{
			{ID: 20, ClientStats: echo},
			{ID: 21, ClientStats: echo},
		}},
	}}
	disabler := &suspendCapture{}
	svc := New(users, &fakeOwnershipRepo{byUser: map[int64][]*domain.XUIClientEntry{}}, &fakeTrafficRepo{}, nil, nil, pool, disabler)
	svc.SetPSPClientRepo(psp)

	if err := svc.PollOnce(context.Background()); err != nil {
		t.Fatalf("PollOnce: %v", err)
	}

	// delta = (700-100) up + (500-100) down = 1000 total, counted ONCE across the
	// two echoing inbounds. A per-inbound sum would yield 2200 (double-count).
	if got := users.users[1].LifetimeTotalBytes; got != 1000 {
		t.Fatalf("user lifetime = %d, want 1000 (single-count across 2 echoing inbounds; >1000 means double-count)", got)
	}
	// periodUsed 1000 >= limit 1000 → service suspended for traffic.
	if disabler.suspendedUser != 1 || disabler.suspendedReason != domain.DisabledTrafficExceeded {
		t.Fatalf("want user 1 service-suspended (traffic exceeded), got user=%d reason=%q",
			disabler.suspendedUser, disabler.suspendedReason)
	}
}

// The migration window: a user can briefly hold BOTH a legacy ownership row and a
// shared client. They are NEVER both active at once — 3X-UI attributes a UUID to
// exactly one client object per inbound, so only one email carries traffic. This
// pins the steady-state after the legacy client is deleted: the panel reports the
// shared email only, and the lingering ownership row (not reported by the panel)
// must contribute NOTHING — Phase 2 + Phase 2b coexist without double-counting.
func TestPollOnce_LegacyAndSharedWindow_NoDoubleCount(t *testing.T) {
	old := time.Now().Add(-24 * time.Hour)
	users := &fakeUserRepo{users: map[int64]*domain.User{
		1: {ID: 1, Enabled: true, LifetimeBaselineAt: &old}, // cutoff set → no bootstrap fold
	}}
	// Legacy ownership row still present (migration not yet torn down) but its
	// 3X-UI client was deleted, so the panel reports no stats for u1-n5@.
	ownership := &fakeOwnershipRepo{byUser: map[int64][]*domain.XUIClientEntry{
		1: {{ID: 1, UserID: 1, PanelID: 10, InboundID: 20, ClientEmail: "u1-n5@psp.local",
			LastRawUpBytes: 50, LastRawDownBytes: 50, LastRawTotalBytes: 100, CreatedAt: old}},
	}}
	// Shared client carries the real traffic (pre-seeded baseline → a delta lands).
	psp := &fakePSPClientRepo{byUser: map[int64][]*domain.PSPClient{
		1: {{ID: 1, UserID: 1, PanelID: 10, Email: "u1@psp.local",
			LastRawUpBytes: 100, LastRawDownBytes: 100, LastRawTotalBytes: 200}},
	}}
	pool := &fakeXUIPool{clients: map[int64]ports.XUIClient{
		10: &fakeXUIClient{inbounds: []ports.Inbound{
			// ONLY the shared email is reported (legacy client already deleted).
			{ID: 20, ClientStats: []ports.ClientTraffic{{Email: "u1@psp.local", Up: 700, Down: 500}}},
		}},
	}}
	svc := New(users, ownership, &fakeTrafficRepo{}, nil, nil, pool, &fakeDisabler{})
	svc.SetPSPClientRepo(psp)

	if err := svc.PollOnce(context.Background()); err != nil {
		t.Fatalf("PollOnce: %v", err)
	}

	// Only the shared delta counts: (700-100)+(500-100) = 1000. The lingering
	// legacy ownership row contributes nothing (the panel never reported it).
	if got := users.users[1].LifetimeTotalBytes; got != 1000 {
		t.Fatalf("user lifetime = %d, want 1000 (shared delta only; >1000 means legacy+shared double-count)", got)
	}
}
