// reset-admin-password rewrites a local-account password hash directly in
// the SQLite DB. Use it when the bootstrap admin password got lost — there
// is no reset-from-UI flow for the admin account itself.
//
// The panel must be stopped before running, otherwise SQLite write locking
// will block.
//
// Usage (from repo root):
//
//	go run ./cmd/reset-admin-password/
//	go run ./cmd/reset-admin-password/ -upn alice -password s3cret
package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/glebarez/sqlite"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func main() {
	dbPath := flag.String("db", "data/panel.db", "path to panel.db (SQLite); panel's default data_dir is ./data relative to its working directory")
	upn := flag.String("upn", "admin", "user UPN to reset / verify")
	password := flag.String("password", "admin", "new password (or password to verify in -verify mode)")
	verify := flag.Bool("verify", false, "do not modify; just compare password against the stored hash")
	flag.Parse()

	if _, err := os.Stat(*dbPath); err != nil {
		fmt.Fprintf(os.Stderr, "db not found: %s\n", *dbPath)
		os.Exit(1)
	}

	g, err := gorm.Open(sqlite.Open(*dbPath), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		fmt.Fprintf(os.Stderr, "open db: %v\n", err)
		os.Exit(1)
	}

	if *verify {
		var row struct {
			UPN          string
			PasswordHash string `gorm:"column:password_hash"`
			Enabled      bool
		}
		if err := g.Raw(`SELECT upn, password_hash, enabled FROM users WHERE upn = ?`, *upn).Scan(&row).Error; err != nil {
			fmt.Fprintf(os.Stderr, "select: %v\n", err)
			os.Exit(1)
		}
		if row.UPN == "" {
			fmt.Fprintf(os.Stderr, "no user found with upn=%s\n", *upn)
			os.Exit(1)
		}
		hashPrefix := row.PasswordHash
		if len(hashPrefix) > 12 {
			hashPrefix = hashPrefix[:12] + "..."
		}
		fmt.Printf("user found: upn=%s, enabled=%v, hash=%s (len=%d)\n",
			row.UPN, row.Enabled, hashPrefix, len(row.PasswordHash))
		if err := bcrypt.CompareHashAndPassword([]byte(row.PasswordHash), []byte(*password)); err != nil {
			fmt.Fprintf(os.Stderr, "VERIFY FAILED: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("VERIFY OK — password=%s matches stored hash\n", *password)
		return
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(*password), bcrypt.DefaultCost)
	if err != nil {
		fmt.Fprintf(os.Stderr, "hash: %v\n", err)
		os.Exit(1)
	}

	res := g.Exec(`UPDATE users SET password_hash = ? WHERE upn = ?`, string(hash), *upn)
	if res.Error != nil {
		fmt.Fprintf(os.Stderr, "update: %v\n", res.Error)
		os.Exit(1)
	}
	if res.RowsAffected == 0 {
		fmt.Fprintf(os.Stderr, "no user found with upn=%s\n", *upn)
		os.Exit(1)
	}
	hashPrefix := string(hash)[:12] + "..."
	fmt.Printf("password reset OK — upn=%s, rows=%d, password=%s, hash=%s\n", *upn, res.RowsAffected, *password, hashPrefix)
}
