// Package seed releases default rulesets and templates into the runtime
// config directory on first launch. The defaults are embedded into the
// binary, so the panel can boot from an empty config dir whether it lives
// on a freshly bind-mounted Docker volume or a clean systemd /opt/psp path.
//
// Existing files in the config dir are NEVER overwritten — admins may have
// customized them and we must preserve that work. To restore a default that
// was deleted, just remove the file and restart the binary.
package seed

import (
	"embed"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
)

// The "all:" prefix includes dotfiles, so hidden default fragments (if any
// are added later) are still picked up.
//
//go:embed all:files
var defaultsFS embed.FS

// Ensure walks the baked-in defaults and writes any file that is missing
// under configDir. Directories are created as needed; existing files are
// left alone.
func Ensure(configDir string) error {
	return fs.WalkDir(defaultsFS, "files", func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if path == "files" {
			return nil
		}
		rel := path[len("files/"):]
		target := filepath.Join(configDir, rel)

		if d.IsDir() {
			return os.MkdirAll(target, 0o755)
		}

		if _, err := os.Stat(target); err == nil {
			return nil // already present — preserve admin edits
		} else if !os.IsNotExist(err) {
			return fmt.Errorf("stat %s: %w", target, err)
		}

		body, err := defaultsFS.ReadFile(path)
		if err != nil {
			return fmt.Errorf("read embed %s: %w", path, err)
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return fmt.Errorf("mkdir %s: %w", filepath.Dir(target), err)
		}
		if err := os.WriteFile(target, body, 0o644); err != nil {
			return fmt.Errorf("write %s: %w", target, err)
		}
		return nil
	})
}
