package rollup

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"gorm.io/gorm"

	"github.com/KazuhaHub/passwall-sub-panel/internal/adapters/mysql"
)

// newServiceFromTest spins up a fresh SQLite under t.TempDir(), runs
// EnsureSchema, and returns a rollup Service hooked up to it plus an
// exec/read helper. Tests run against the real GORM dialect so the
// OnConflict upsert and uniqueIndex shapes behave like production.
func newServiceFromTest(t *testing.T) (*Service, *gorm.DB) {
	t.Helper()
	g, err := mysql.Open("sqlite", filepath.Join(t.TempDir(), "panel.db"))
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := mysql.EnsureSchema(g); err != nil {
		t.Fatalf("ensure schema: %v", err)
	}
	t.Cleanup(func() {
		sqlDB, err := g.DB()
		if err == nil {
			_ = sqlDB.Close()
		}
	})
	return New(g), g
}

func insertUserSnap(t *testing.T, g *gorm.DB, userID int64, ts time.Time, up, down, total int64) {
	t.Helper()
	if err := g.Exec(
		`INSERT INTO traffic_snapshots(user_id, up_bytes, down_bytes, total_bytes, captured_at) VALUES(?,?,?,?,?)`,
		userID, up, down, total, ts,
	).Error; err != nil {
		t.Fatalf("insert raw user snap: %v", err)
	}
}

func insertNodeSnap(t *testing.T, g *gorm.DB, nodeID int64, ts time.Time, up, down, total int64) {
	t.Helper()
	if err := g.Exec(
		`INSERT INTO node_traffic_snapshots(node_id, up_bytes, down_bytes, total_bytes, captured_at) VALUES(?,?,?,?,?)`,
		nodeID, up, down, total, ts,
	).Error; err != nil {
		t.Fatalf("insert raw node snap: %v", err)
	}
}

func insertClientSnap(t *testing.T, g *gorm.DB, userID, panelID int64, inboundID int, email string, ts time.Time, up, down, total int64) {
	t.Helper()
	if err := g.Exec(
		`INSERT INTO client_traffic_snapshots(user_id, panel_id, inbound_id, client_email, up_bytes, down_bytes, total_bytes, captured_at) VALUES(?,?,?,?,?,?,?,?)`,
		userID, panelID, inboundID, email, up, down, total, ts,
	).Error; err != nil {
		t.Fatalf("insert raw client snap: %v", err)
	}
}

// scanOne pulls a single row into dest via GORM Raw. Used because the
// hourly tables don't have a registered repo — we want to assert on raw
// column values directly.
func scanOne(t *testing.T, g *gorm.DB, dest any, query string, args ...any) {
	t.Helper()
	if err := g.Raw(query, args...).Scan(dest).Error; err != nil {
		t.Fatalf("scan %q: %v", query, err)
	}
}

func countRows(t *testing.T, g *gorm.DB, query string, args ...any) int64 {
	t.Helper()
	var n int64
	if err := g.Raw(query, args...).Scan(&n).Error; err != nil {
		t.Fatalf("count %q: %v", query, err)
	}
	return n
}

// Reference times shared across tests. h14 is far enough in the past
// that hourFloor(now) is well beyond it, so it always counts as a
// completed hour.
var (
	h14 = time.Date(2025, 5, 17, 14, 0, 0, 0, time.UTC)
	h15 = time.Date(2025, 5, 17, 15, 0, 0, 0, time.UTC)
)

// TestRollupUserDeltaWithinBucket: 4 raw rows in the 14:00 UTC hour for
// user 1 form a monotonic ramp 100→500. After rollup, the 14:00 hourly
// row should store the delta (400).
func TestRollupUserDeltaWithinBucket(t *testing.T) {
	svc, g := newServiceFromTest(t)
	ctx := context.Background()

	insertUserSnap(t, g, 1, h14.Add(5*time.Minute), 100, 50, 150)
	insertUserSnap(t, g, 1, h14.Add(10*time.Minute), 200, 100, 300)
	insertUserSnap(t, g, 1, h14.Add(40*time.Minute), 350, 175, 525)
	insertUserSnap(t, g, 1, h14.Add(55*time.Minute), 500, 250, 750)

	if err := svc.RollupOnce(ctx); err != nil {
		t.Fatalf("rollup: %v", err)
	}

	var got struct {
		UpBytes    int64
		DownBytes  int64
		TotalBytes int64
	}
	scanOne(t, g, &got,
		"SELECT up_bytes, down_bytes, total_bytes FROM traffic_snapshots_hourly WHERE user_id = ? AND bucket_start = ?",
		1, h14)
	if got.UpBytes != 400 || got.DownBytes != 200 || got.TotalBytes != 600 {
		t.Fatalf("delta = up=%d down=%d total=%d, want 400/200/600", got.UpBytes, got.DownBytes, got.TotalBytes)
	}
}

// TestRollupOnlyCompletedHours: a raw row captured in the CURRENT hour
// must NOT be rolled up (would produce a wrong-low partial delta).
func TestRollupOnlyCompletedHours(t *testing.T) {
	svc, g := newServiceFromTest(t)
	ctx := context.Background()

	insertUserSnap(t, g, 1, h14.Add(5*time.Minute), 100, 50, 150)
	insertUserSnap(t, g, 1, h14.Add(55*time.Minute), 500, 250, 750)
	insertUserSnap(t, g, 1, h15.Add(10*time.Minute), 600, 300, 900)
	insertUserSnap(t, g, 1, h15.Add(50*time.Minute), 800, 400, 1200)
	// "Current hour" — captured AFTER hourFloor(now), so cutoff < captured_at.
	insertUserSnap(t, g, 1, time.Now().Add(time.Hour), 9999, 9999, 9999)

	if err := svc.RollupOnce(ctx); err != nil {
		t.Fatalf("rollup: %v", err)
	}

	if n := countRows(t, g,
		"SELECT COUNT(*) FROM traffic_snapshots_hourly WHERE user_id = ? AND bucket_start = ?",
		1, hourFloor(time.Now().Add(time.Hour))); n != 0 {
		t.Fatalf("future-hour row should not be rolled up, found %d hourly rows", n)
	}
	if n := countRows(t, g, "SELECT COUNT(*) FROM traffic_snapshots_hourly WHERE user_id = ?", 1); n != 2 {
		t.Fatalf("expected 2 completed hour buckets (14h + 15h), got %d", n)
	}
}

// TestRollupIdempotent: running rollup three times on the same raw data
// must produce a single hourly row, not three. This is what makes "first
// run = backfill" safe even if rollup retries because of, say, the cron
// pricking it after a panel restart.
func TestRollupIdempotent(t *testing.T) {
	svc, g := newServiceFromTest(t)
	ctx := context.Background()

	insertUserSnap(t, g, 1, h14.Add(5*time.Minute), 100, 50, 150)
	insertUserSnap(t, g, 1, h14.Add(55*time.Minute), 500, 250, 750)

	for i := 0; i < 3; i++ {
		if err := svc.RollupOnce(ctx); err != nil {
			t.Fatalf("rollup iter %d: %v", i, err)
		}
	}

	if n := countRows(t, g, "SELECT COUNT(*) FROM traffic_snapshots_hourly"); n != 1 {
		t.Fatalf("expected exactly 1 hourly row after 3 rollups, got %d", n)
	}
	var got struct{ UpBytes int64 }
	scanOne(t, g, &got, "SELECT up_bytes FROM traffic_snapshots_hourly LIMIT 1")
	if got.UpBytes != 400 {
		t.Fatalf("up delta = %d, want 400", got.UpBytes)
	}
}

// TestRollupClientAndNode: the client and node code paths use a wider
// unique-key shape (panel_id, inbound_id, client_email, bucket_start)
// vs (node_id, bucket_start). Make sure both rollup correctly.
func TestRollupClientAndNode(t *testing.T) {
	svc, g := newServiceFromTest(t)
	ctx := context.Background()

	insertClientSnap(t, g, 1, 10, 2, "alice@ex.com", h14.Add(5*time.Minute), 100, 50, 150)
	insertClientSnap(t, g, 1, 10, 2, "alice@ex.com", h14.Add(55*time.Minute), 500, 250, 750)

	insertNodeSnap(t, g, 7, h14.Add(5*time.Minute), 100, 50, 150)
	insertNodeSnap(t, g, 7, h14.Add(55*time.Minute), 500, 250, 750)

	if err := svc.RollupOnce(ctx); err != nil {
		t.Fatalf("rollup: %v", err)
	}

	var got struct{ UpBytes int64 }
	scanOne(t, g, &got,
		"SELECT up_bytes FROM client_traffic_snapshots_hourly WHERE panel_id=? AND inbound_id=? AND client_email=?",
		10, 2, "alice@ex.com")
	if got.UpBytes != 400 {
		t.Fatalf("client delta = %d, want 400", got.UpBytes)
	}

	scanOne(t, g, &got,
		"SELECT up_bytes FROM node_traffic_snapshots_hourly WHERE node_id=?", 7)
	if got.UpBytes != 400 {
		t.Fatalf("node delta = %d, want 400", got.UpBytes)
	}
}

// TestHourFloor exercises the bucket-boundary math: any timestamp within
// a UTC hour truncates to the start of that hour, sub-second precision
// is dropped, and non-UTC inputs convert to UTC before truncating.
func TestHourFloor(t *testing.T) {
	la, err := time.LoadLocation("America/Los_Angeles")
	if err != nil {
		t.Fatalf("load LA tz: %v", err)
	}
	cases := []struct {
		name string
		in   time.Time
		want time.Time
	}{
		{"top of hour utc", time.Date(2026, 5, 17, 14, 0, 0, 0, time.UTC), time.Date(2026, 5, 17, 14, 0, 0, 0, time.UTC)},
		{"mid hour utc", time.Date(2026, 5, 17, 14, 37, 42, 123456789, time.UTC), time.Date(2026, 5, 17, 14, 0, 0, 0, time.UTC)},
		{"LA noon converts to 19:00 UTC", time.Date(2026, 5, 17, 12, 30, 0, 0, la), time.Date(2026, 5, 17, 19, 0, 0, 0, time.UTC)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := hourFloor(tc.in)
			if !got.Equal(tc.want) {
				t.Fatalf("hourFloor(%v) = %v, want %v", tc.in, got, tc.want)
			}
		})
	}
}
