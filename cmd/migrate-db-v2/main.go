// migrate-db-v2 is a one-shot side-by-side migration from the v2 schema
// (panel) to the v3 schema (panel_v3). It is a separate Go binary — the
// main panel program does NOT know about the old schema, by design (the
// "Cloudreve upgrade" pattern: V3 only reads the new database, leaving the
// old DB completely untouched as a permanent backup).
//
// Lifecycle:
//
//  1. Build & run this CLI once with --src=<old-db> --dst=<new-db>.
//  2. Verify the new DB drives the panel correctly.
//  3. Delete the cmd/migrate-db-v2/ directory in the same commit as the
//     migration sign-off — git history is the audit trail.
//
// Usage (from repo root):
//
//	# SQLite-to-SQLite (local dev):
//	go run ./cmd/migrate-db-v2/ --driver=sqlite \
//	    --src=data/panel.db --dst=data/panel_v3.db
//
//	# MySQL-to-MySQL (production):
//	go run ./cmd/migrate-db-v2/ --driver=mysql \
//	    --src='user:pass@tcp(host:3306)/panel?charset=utf8mb4&parseTime=true' \
//	    --dst='user:pass@tcp(host:3306)/panel_v3?charset=utf8mb4&parseTime=true'
//
// On success the program prints next-steps (edit config.yaml's `database`
// field to point at the new DB, restart the panel).
//
// Re-running is safe: the program refuses to start if the destination
// already has any settings rows (a v3 DB has at minimum the seeded KV).
// To re-run during a botched migration, DROP DATABASE panel_v3 in MySQL
// (or delete the v3 SQLite file) and start over — the source is never
// modified.
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	sqlitedriver "github.com/glebarez/sqlite"
	mysqldriver "gorm.io/driver/mysql"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"

	"github.com/KazuhaHub/passwall-sub-panel/internal/adapters/mysql"
)

func main() {
	driver := flag.String("driver", "sqlite", "db driver: sqlite | mysql (both src and dst share the driver)")
	src := flag.String("src", "", "source DB: SQLite path or MySQL DSN")
	dst := flag.String("dst", "", "destination DB: SQLite path or MySQL DSN (must already exist for mysql)")
	dryRun := flag.Bool("dry-run", false, "open both DBs, report what would be copied, but do not write to dst")
	flag.Parse()

	// Note: no --secret flag. AES-GCM-encrypted columns (mail_settings.smtp_password,
	// saml_settings.sp_key_pem, oidc_settings.client_secret, xui_panels.api_token /
	// password) carry their ciphertext as opaque strings through this migration
	// — we never decrypt or re-encrypt. The v3.0.0 panel decrypts at Load
	// time using config.yaml's SecretKeyMaterial, which must match what the
	// legacy panel used.
	// If it doesn't, the panel surfaces a clear "decrypt secret" error on boot
	// and the operator fixes config.yaml without touching this cmd.

	if *src == "" || *dst == "" {
		fmt.Fprintln(os.Stderr, "ERROR: --src and --dst are both required")
		flag.Usage()
		os.Exit(2)
	}

	srcDB, err := openDB(*driver, *src)
	if err != nil {
		log.Fatalf("open src: %v", err)
	}
	dstDB, err := openDB(*driver, *dst)
	if err != nil {
		log.Fatalf("open dst: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancel()

	if err := guardDstEmpty(ctx, dstDB); err != nil {
		log.Fatalf("destination not safe to migrate into: %v", err)
	}

	if !*dryRun {
		if err := mysql.EnsureSchema(dstDB); err != nil {
			log.Fatalf("create v3 schema on dst: %v", err)
		}
	}

	plan, err := buildMigrationPlan(ctx, srcDB)
	if err != nil {
		log.Fatalf("scan src: %v", err)
	}
	plan.print()

	if *dryRun {
		fmt.Println("\nDRY RUN — no rows written. Re-run without --dry-run to apply.")
		return
	}

	if err := runMigration(ctx, srcDB, dstDB, plan); err != nil {
		log.Fatalf("migration failed: %v", err)
	}

	fmt.Println()
	fmt.Println("Migration complete.")
	fmt.Println()
	fmt.Println("Next steps:")
	fmt.Println("  1. Update config.yaml's `database` field to point at the dst DB.")
	fmt.Println("  2. Restart the panel and verify everything works.")
	fmt.Println("  3. Keep the old DB around for a week as backup, then drop it.")
	fmt.Println("  4. Delete cmd/migrate-db-v2/ in your next commit — its job is done.")
}

func openDB(driver, dsn string) (*gorm.DB, error) {
	cfg := &gorm.Config{
		Logger: gormlogger.New(log.New(os.Stderr, "[gorm] ", log.LstdFlags), gormlogger.Config{
			SlowThreshold:             5 * time.Second,
			LogLevel:                  gormlogger.Warn,
			IgnoreRecordNotFoundError: true,
		}),
	}
	switch driver {
	case "sqlite":
		if dir := filepath.Dir(dsn); dir != "" && dir != "." {
			if err := os.MkdirAll(dir, 0o755); err != nil {
				return nil, fmt.Errorf("mkdir for sqlite: %w", err)
			}
		}
		return gorm.Open(sqlitedriver.Open(dsn), cfg)
	case "mysql":
		return gorm.Open(mysqldriver.Open(dsn), cfg)
	default:
		return nil, fmt.Errorf("unknown driver %q", driver)
	}
}

// guardDstEmpty refuses to run if the destination already has any settings
// rows. The migration seeds a sentinel row (`type='_migration'`) BEFORE any
// other dst writes, so a freshly-created DB is the only state with empty
// settings — any other state (sentinel present, partial copy, full success,
// or hand-edited) means re-running would either double-insert or stomp on
// admin's work. Operator's recovery path is "DROP DATABASE; CREATE; re-run",
// matching the side-by-side design where the source DB is untouched.
func guardDstEmpty(ctx context.Context, dst *gorm.DB) error {
	// Settings table may not exist yet on a brand-new DB — that's fine.
	if !dst.Migrator().HasTable("settings") {
		return nil
	}
	var count int64
	if err := dst.WithContext(ctx).Table("settings").Count(&count).Error; err != nil {
		return fmt.Errorf("count settings on dst: %w", err)
	}
	if count > 0 {
		return fmt.Errorf("dst has %d rows in `settings` already — drop the destination DB and re-run", count)
	}
	return nil
}
