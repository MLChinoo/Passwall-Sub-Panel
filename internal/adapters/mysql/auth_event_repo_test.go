package mysql

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/KazuhaHub/passwall-sub-panel/internal/domain"
	"github.com/KazuhaHub/passwall-sub-panel/internal/ports"
)

func TestAuthEventRepo(t *testing.T) {
	db, err := Open("sqlite", filepath.Join(t.TempDir(), "panel.db"))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	if err := EnsureSchema(db); err != nil {
		t.Fatalf("schema: %v", err)
	}
	// Close the SQLite handle before t.TempDir cleanup, else Windows can't
	// unlink the still-open panel.db.
	t.Cleanup(func() {
		if sqlDB, derr := db.DB(); derr == nil {
			_ = sqlDB.Close()
		}
	})
	repo := NewRepos(db).AuthEvent
	ctx := context.Background()
	now := time.Now()

	seed := []*domain.AuthEvent{
		{UserID: 1, UPN: "alice@x", Method: domain.AuthMethodLocal, Outcome: domain.AuthOutcomeSuccess, IP: "1.1.1.1", At: now.Add(-1 * time.Hour)},
		{UserID: 0, UPN: "alice@x", Method: domain.AuthMethodLocal, Outcome: domain.AuthOutcomeFailure, Reason: "invalid_credentials", IP: "2.2.2.2", At: now.Add(-2 * time.Hour)},
		{UserID: 2, UPN: "bob@x", Method: domain.AuthMethodSAML, Outcome: domain.AuthOutcomeSuccess, IP: "3.3.3.3", At: now.Add(-3 * time.Hour)},
		{UserID: 3, UPN: "old@x", Method: domain.AuthMethodOIDC, Outcome: domain.AuthOutcomeSuccess, IP: "4.4.4.4", At: now.AddDate(0, 0, -100)}, // older than 90d
	}
	for _, e := range seed {
		if err := repo.Insert(ctx, e); err != nil {
			t.Fatalf("insert: %v", err)
		}
		if e.ID == 0 {
			t.Fatal("insert did not set ID")
		}
	}

	page := ports.Pagination{Page: 1, PageSize: 50}
	total := func(f ports.AuthEventFilter) int64 {
		_, tot, err := repo.List(ctx, f)
		if err != nil {
			t.Fatalf("list: %v", err)
		}
		return tot
	}

	if got := total(ports.AuthEventFilter{Pagination: page}); got != 4 {
		t.Fatalf("list all total = %d, want 4", got)
	}
	// Default order is at DESC → newest (alice success) first.
	if items, _, _ := repo.List(ctx, ports.AuthEventFilter{Pagination: page}); items[0].UPN != "alice@x" || items[0].Outcome != domain.AuthOutcomeSuccess {
		t.Fatalf("newest-first ordering broken: got %+v", items[0])
	}
	if got := total(ports.AuthEventFilter{Pagination: page, Outcome: "failure"}); got != 1 {
		t.Fatalf("outcome=failure total = %d, want 1", got)
	}
	if got := total(ports.AuthEventFilter{Pagination: page, Method: "saml"}); got != 1 {
		t.Fatalf("method=saml total = %d, want 1", got)
	}
	uid := int64(1)
	if got := total(ports.AuthEventFilter{Pagination: page, UserID: &uid}); got != 1 {
		t.Fatalf("user_id=1 total = %d, want 1", got)
	}
	if got := total(ports.AuthEventFilter{Pagination: page, Search: "bob"}); got != 1 {
		t.Fatalf("search upn=bob total = %d, want 1", got)
	}
	if got := total(ports.AuthEventFilter{Pagination: page, Search: "invalid_credentials"}); got != 1 {
		t.Fatalf("search reason total = %d, want 1", got)
	}

	// Retention: prune everything older than 90d → drops the 100-day-old row.
	deleted, err := repo.DeleteBefore(ctx, now.AddDate(0, 0, -90))
	if err != nil || deleted != 1 {
		t.Fatalf("DeleteBefore = %d (err %v), want 1", deleted, err)
	}
	if got := total(ports.AuthEventFilter{Pagination: page}); got != 3 {
		t.Fatalf("after prune total = %d, want 3", got)
	}
}
