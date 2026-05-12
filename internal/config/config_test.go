package config

import (
	"path/filepath"
	"testing"
)

func TestDatabaseDefaultsToSQLiteWhenMySQLUnset(t *testing.T) {
	cfg := Config{DataDir: "./data"}
	cfg.applyDefaults()

	if got := cfg.DBKind(); got != "sqlite" {
		t.Fatalf("DBKind() = %q, want sqlite", got)
	}
	if got, want := cfg.DBDSN(), filepath.Join("./data", "panel.db"); got != want {
		t.Fatalf("DBDSN() = %q, want %q", got, want)
	}
}

func TestDatabaseUsesMySQLWhenDSNConfigured(t *testing.T) {
	cfg := Config{
		MySQL: MySQLConfig{
			DSN: "user:pass@tcp(127.0.0.1:3306)/passwall",
		},
	}
	cfg.applyDefaults()

	if got := cfg.DBKind(); got != "mysql" {
		t.Fatalf("DBKind() = %q, want mysql", got)
	}
	if got, want := cfg.DBDSN(), cfg.MySQL.DSN; got != want {
		t.Fatalf("DBDSN() = %q, want %q", got, want)
	}
}

func TestDatabaseUsesExplicitSQLiteDSN(t *testing.T) {
	cfg := Config{
		MySQL: MySQLConfig{
			DSN: "sqlite:./custom/panel.db",
		},
	}
	cfg.applyDefaults()

	if got := cfg.DBKind(); got != "sqlite" {
		t.Fatalf("DBKind() = %q, want sqlite", got)
	}
	if got, want := cfg.DBDSN(), "./custom/panel.db"; got != want {
		t.Fatalf("DBDSN() = %q, want %q", got, want)
	}
}
