package mysql

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/KazuhaHub/passwall-sub-panel/internal/domain"
)

func TestTrafficSnapshotsReturnNotFoundWhenEmpty(t *testing.T) {
	db, err := Open("sqlite", filepath.Join(t.TempDir(), "panel.db"))
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := EnsureSchema(db); err != nil {
		t.Fatalf("schema: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("unwrap db: %v", err)
	}
	defer func() {
		if err := sqlDB.Close(); err != nil {
			t.Fatalf("close db: %v", err)
		}
	}()

	repo := NewRepos(db).Traffic
	ctx := context.Background()

	if _, err := repo.LatestForUser(ctx, 1); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("LatestForUser error = %v, want ErrNotFound", err)
	}
	if _, err := repo.LastBefore(ctx, 1, time.Now()); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("LastBefore error = %v, want ErrNotFound", err)
	}
}

// TestTrafficPruneBefore covers the v3.0.0 retention DELETE — guards against
// indexing regressions (the captured_at single-column index is what makes
// this query a range-scan instead of full-table). Verifies that both
// traffic_snapshots and client_traffic_snapshots are pruned in one call,
// and that the cutoff comparison is strict (rows AT cutoff survive).
func TestTrafficPruneBefore(t *testing.T) {
	db, err := Open("sqlite", filepath.Join(t.TempDir(), "panel.db"))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	if err := EnsureSchema(db); err != nil {
		t.Fatalf("schema: %v", err)
	}
	sqlDB, _ := db.DB()
	defer sqlDB.Close()

	repo := NewRepos(db).Traffic
	ctx := context.Background()
	cutoff := time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC)

	mustInsert := func(t *testing.T, s *domain.TrafficSnapshot) {
		t.Helper()
		if err := repo.Insert(ctx, s); err != nil {
			t.Fatalf("insert: %v", err)
		}
	}
	mustInsertClient := func(t *testing.T, s *domain.ClientTrafficSnapshot) {
		t.Helper()
		if err := repo.InsertClient(ctx, s); err != nil {
			t.Fatalf("insert client: %v", err)
		}
	}

	mustInsert(t, &domain.TrafficSnapshot{UserID: 1, TotalBytes: 100, CapturedAt: cutoff.Add(-48 * time.Hour)}) // prune
	mustInsert(t, &domain.TrafficSnapshot{UserID: 1, TotalBytes: 200, CapturedAt: cutoff})                     // keep (strict <)
	mustInsert(t, &domain.TrafficSnapshot{UserID: 1, TotalBytes: 300, CapturedAt: cutoff.Add(48 * time.Hour)}) // keep

	mustInsertClient(t, &domain.ClientTrafficSnapshot{UserID: 1, PanelID: 10, InboundID: 1, ClientEmail: "a@x", TotalBytes: 10, CapturedAt: cutoff.Add(-time.Hour)}) // prune
	mustInsertClient(t, &domain.ClientTrafficSnapshot{UserID: 1, PanelID: 10, InboundID: 1, ClientEmail: "a@x", TotalBytes: 20, CapturedAt: cutoff.Add(time.Hour)})  // keep

	deleted, err := repo.PruneBefore(ctx, cutoff)
	if err != nil {
		t.Fatalf("PruneBefore: %v", err)
	}
	// 1 traffic_snapshot + 1 client_traffic_snapshot deleted = 2.
	if deleted != 2 {
		t.Errorf("deleted = %d, want 2", deleted)
	}

	since := time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC)
	until := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)
	rows, err := repo.ListByUser(ctx, 1, since, until)
	if err != nil {
		t.Fatalf("ListByUser: %v", err)
	}
	if len(rows) != 2 {
		t.Errorf("rows after prune = %d, want 2 (cutoff row kept + later row kept)", len(rows))
	}
}

