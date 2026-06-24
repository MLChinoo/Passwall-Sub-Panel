package app

import "testing"

// While migration is incomplete, the heavy shared-client heal runs every reconcile
// tick (converge fast). Once complete it drops to a slow drift backstop — every Nth
// tick — since event-driven resync already keeps steady state correct.
func TestShouldRunSharedHeal(t *testing.T) {
	// Migrating: every tick heals, regardless of tick number.
	for tick := 1; tick <= 6; tick++ {
		if !shouldRunSharedHeal(tick, false) {
			t.Fatalf("tick %d while migrating must run the heal", tick)
		}
	}
	// Complete: heal only on multiples of the backstop interval.
	runs := 0
	for tick := 1; tick <= sharedHealBackstopEvery*3; tick++ {
		if shouldRunSharedHeal(tick, true) {
			runs++
			if tick%sharedHealBackstopEvery != 0 {
				t.Fatalf("complete: tick %d should NOT heal (not a multiple of %d)", tick, sharedHealBackstopEvery)
			}
		}
	}
	if runs != 3 {
		t.Fatalf("complete: want 3 heals over %d ticks, got %d", sharedHealBackstopEvery*3, runs)
	}
}
