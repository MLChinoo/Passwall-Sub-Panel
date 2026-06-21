package traffic

import (
	"context"
	"testing"

	"github.com/KazuhaHub/passwall-sub-panel/internal/domain"
)

// recordSharedClientStats must seed (zero delta, no lifetime advance) on the
// FIRST observation so a shared client read mid-stream can't spike the user's
// quota, then report real monotonic deltas, then no-op when idle.
func TestRecordSharedClientStats_SeedThenDeltaThenIdle(t *testing.T) {
	s := &Service{}
	sink := &pollSink{}
	c := &domain.PSPClient{ID: 1}

	// First observation with a non-zero counter → seed only.
	d := s.recordSharedClientStats(context.Background(), c, 100, 50, sink)
	if d.up != 0 || d.down != 0 || d.total != 0 || d.hadPrev {
		t.Fatalf("first obs must seed with zero delta: %+v", d)
	}
	if c.LifetimeTotalBytes != 0 {
		t.Fatalf("first obs must NOT advance lifetime, got %d", c.LifetimeTotalBytes)
	}
	if c.LastRawUpBytes != 100 || c.LastRawDownBytes != 50 || c.LastRawTotalBytes != 150 {
		t.Fatalf("first obs must set the raw baseline: %+v", c)
	}

	// Second observation → real delta, lifetime advances by exactly the delta.
	d = s.recordSharedClientStats(context.Background(), c, 180, 70, sink)
	if d.up != 80 || d.down != 20 || d.total != 100 || !d.hadPrev {
		t.Fatalf("delta = %+v, want up80 down20 total100 hadPrev=true", d)
	}
	if c.LifetimeUpBytes != 80 || c.LifetimeDownBytes != 20 || c.LifetimeTotalBytes != 100 {
		t.Fatalf("lifetime must advance by the delta: %+v", c)
	}

	// Same counter again → idle no-op (no further lifetime change).
	d = s.recordSharedClientStats(context.Background(), c, 180, 70, sink)
	if d.up != 0 || d.down != 0 || d.total != 0 {
		t.Fatalf("idle must be a no-op delta, got %+v", d)
	}
	if c.LifetimeTotalBytes != 100 {
		t.Fatalf("idle must not change lifetime, got %d", c.LifetimeTotalBytes)
	}
}

// A genuinely-idle client (0/0) on first sight writes nothing at all.
func TestRecordSharedClientStats_IdleZeroFirstObsNoWrite(t *testing.T) {
	s := &Service{}
	sink := &pollSink{}
	d := s.recordSharedClientStats(context.Background(), &domain.PSPClient{ID: 2}, 0, 0, sink)
	if d.up != 0 || d.down != 0 || d.total != 0 || d.hadPrev {
		t.Fatalf("idle-zero first obs must be a pure no-op: %+v", d)
	}
	if len(sink.pspClientUpdates) != 0 {
		t.Fatalf("idle-zero first obs must not queue a counter write, got %d", len(sink.pspClientUpdates))
	}
}
