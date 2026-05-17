// Package paneltz centralizes "what timezone does the panel use for
// system-level time math" lookups. The settings table stores the IANA
// name; every place that needs a calendar-day boundary for monthly /
// quarterly traffic rolls, user expire_at math, or the default zone
// for admin-side chart bucketing pulls it through here.
//
// User-facing views (subscription page, /user/me dashboard) intentionally
// stay on the browser's timezone and don't go through this package.
package paneltz

import (
	"context"
	"strings"
	"time"

	"github.com/KazuhaHub/passwall-sub-panel/internal/ports"
)

// Location resolves the configured panel timezone. Falls back to
// time.Local when the settings repo is nil, the load errors out, the
// configured value is blank, or it's unparseable — matching pre-tz
// behavior so existing installs keep working unchanged.
func Location(ctx context.Context, settings ports.SettingsRepo) *time.Location {
	if settings == nil {
		return time.Local
	}
	cfg, err := settings.Load(ctx, ports.UISettings{})
	if err != nil || strings.TrimSpace(cfg.Timezone) == "" {
		return time.Local
	}
	if l, lerr := time.LoadLocation(cfg.Timezone); lerr == nil {
		return l
	}
	return time.Local
}

// Now returns time.Now() in the configured panel timezone.
func Now(ctx context.Context, settings ports.SettingsRepo) time.Time {
	return time.Now().In(Location(ctx, settings))
}

// defaultPanelConcurrency is the fallback fan-out cap used when the
// admin hasn't configured one. 8 is comfortable for the typical 1-5
// panel deployment and large enough that single-panel installs (where
// concurrency is a no-op anyway) don't notice.
const defaultPanelConcurrency = 8

// maxPanelConcurrencyCeiling clamps a misconfigured very-high value so
// a typo in the settings UI ("80" instead of "8") can't slam 3X-UI
// with 80 simultaneous HTTP requests.
const maxPanelConcurrencyCeiling = 64

// ResolveMaxPanelConcurrency turns the raw settings integer into the
// usable concurrency cap for traffic poll / reconcile fan-out: <= 0
// falls back to the default, values above the ceiling clamp down.
// Centralized so both PollOnce and RunOnce agree on bounds.
func ResolveMaxPanelConcurrency(raw int) int {
	if raw <= 0 {
		return defaultPanelConcurrency
	}
	if raw > maxPanelConcurrencyCeiling {
		return maxPanelConcurrencyCeiling
	}
	return raw
}

// Validate checks that a tz string is something this binary's tzdata can
// resolve. Empty is allowed (it means "use server local"). Used by the
// admin settings PUT handler to reject saves the browser offered but Go
// can't actually load — otherwise the save succeeds silently and
// Location's fallback path leaves the admin thinking their pick stuck.
func Validate(tz string) error {
	tz = strings.TrimSpace(tz)
	if tz == "" {
		return nil
	}
	_, err := time.LoadLocation(tz)
	return err
}
