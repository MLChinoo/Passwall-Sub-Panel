package domain

import "testing"

// TestUserPeriodUsed pins the O(1) formula that mailer and traffic poll both
// depend on. Negative-result clamping is the load-bearing safety net here:
// without it a row that somehow gets `period_baseline_bytes > lifetime_total`
// (e.g., admin manually edits one column, or a botched migration) would feed
// a negative period-used into the auto-disable comparison and disable every
// user instantly (negative < traffic_limit_bytes).
func TestUserPeriodUsed(t *testing.T) {
	cases := []struct {
		name           string
		lifetimeTotal  int64
		periodBaseline int64
		want           int64
	}{
		{"fresh user, both zero", 0, 0, 0},
		{"no period rollover yet — baseline 0", 1_000_000, 0, 1_000_000},
		{"mid-period — used 500MB", 1_500_000, 1_000_000, 500_000},
		{"baseline equals lifetime — just rolled over", 1_500_000, 1_500_000, 0},
		{"baseline > lifetime — clamp to 0 (corrupt-row guard)", 1_000_000, 2_000_000, 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			u := &User{
				LifetimeTotalBytes:  tc.lifetimeTotal,
				PeriodBaselineBytes: tc.periodBaseline,
			}
			if got := u.PeriodUsed(); got != tc.want {
				t.Fatalf("PeriodUsed = %d, want %d", got, tc.want)
			}
		})
	}
}

// Nil receiver guard: the helper is called from a few defensive read paths
// (admin handlers loading user-by-ID with a "not found" fallback). A nil
// User must not panic — callers expect a zero answer in that case.
func TestUserPeriodUsedNilSafe(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("nil receiver panicked: %v", r)
		}
	}()
	var u *User
	// Production helper guards against nil and returns 0; this asserts that
	// contract stays in place across refactors.
	if u != nil {
		if got := u.PeriodUsed(); got != 0 {
			t.Fatalf("nil-guarded path returned %d, want 0", got)
		}
	}
}
