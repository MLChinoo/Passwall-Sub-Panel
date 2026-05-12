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
	if err := Migrate(db); err != nil {
		t.Fatalf("migrate: %v", err)
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
