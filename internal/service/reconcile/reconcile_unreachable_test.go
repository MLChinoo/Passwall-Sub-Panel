package reconcile

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/KazuhaHub/passwall-sub-panel/internal/domain"
)

// When a panel's prefetch failed, the panel_unreachable Issue must surface the
// REAL cause (DNS / auth / TLS / 404), not a generic "could not list inbounds" —
// otherwise an admin who moved a node to a new server can't tell whether it's the
// new server's config or PSP.
func TestCheckNodes_PanelUnreachableSurfacesRealError(t *testing.T) {
	node := &domain.Node{ID: 1, PanelID: 7, InboundID: 3, Enabled: true}
	svc := &Service{nodes: &recNodeRepo{nodes: []*domain.Node{node}}, pool: recPool{c: &recClient{}}}
	report := &Report{}

	// Empty cache → panel 7 unreachable; prefetchErrs carries the actual failure.
	errs := map[int64]error{7: errors.New("dial tcp: lookup new-host.example: no such host")}
	svc.checkNodes(context.Background(), report, map[inboundCacheKey]*inboundCacheEntry{}, errs)

	var found *Issue
	for i := range report.Issues {
		if report.Issues[i].Code == "panel_unreachable" {
			found = &report.Issues[i]
		}
	}
	if found == nil {
		t.Fatal("expected a panel_unreachable issue for the empty-cache panel")
	}
	if !strings.Contains(found.Detail, "no such host") {
		t.Fatalf("panel_unreachable detail must surface the real prefetch error, got %q", found.Detail)
	}
}

// A panel that is reachable but whose ListInbounds returns ZERO inbounds (fresh/
// empty new server, or a token without access) leaves nothing in the cache and
// yet has NO fetch error — prefetchInbounds must still record a reason so the
// panel_unreachable Issue isn't a content-free generic string.
func TestPrefetchInbounds_EmptyListIsCaptured(t *testing.T) {
	cache := map[inboundCacheKey]*inboundCacheEntry{}
	errs := prefetchInbounds(context.Background(),
		recPool{c: &recClient{inbounds: nil}}, // ListInbounds → (empty, nil)
		map[int64]struct{}{7: {}}, cache, 1)
	if errs[7] == nil || !strings.Contains(errs[7].Error(), "0 inbounds") {
		t.Fatalf("empty ListInbounds for panel 7 must be captured as a reason, got %v", errs[7])
	}
}
