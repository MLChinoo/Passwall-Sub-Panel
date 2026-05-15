package mysql

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/KazuhaHub/passwall-sub-panel/internal/domain"
)

func TestXUIPanelSecretsEncryptedAtRest(t *testing.T) {
	ConfigureSecretKey("test-db-secret")
	t.Cleanup(func() { ConfigureSecretKey("") })

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

	repo := NewRepos(db).XUIPanel
	ctx := context.Background()
	panel := &domain.XUIPanel{
		Name:     "main",
		URL:      "https://xui.example.test",
		APIToken: "api-token",
		Username: "admin",
		Password: "panel-password",
	}
	if err := repo.Save(ctx, panel); err != nil {
		t.Fatalf("save panel: %v", err)
	}

	var row xuiPanelRow
	if err := db.First(&row, panel.ID).Error; err != nil {
		t.Fatalf("read raw row: %v", err)
	}
	if !strings.HasPrefix(row.APIToken, secretPrefix) || row.APIToken == panel.APIToken {
		t.Fatalf("api token not encrypted at rest: %q", row.APIToken)
	}
	if !strings.HasPrefix(row.Password, secretPrefix) || row.Password == panel.Password {
		t.Fatalf("password not encrypted at rest: %q", row.Password)
	}

	got, err := repo.GetByID(ctx, panel.ID)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if got.APIToken != panel.APIToken || got.Password != panel.Password {
		t.Fatalf("decrypted panel = %#v, want api/password plaintext", got)
	}
}
