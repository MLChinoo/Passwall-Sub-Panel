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
