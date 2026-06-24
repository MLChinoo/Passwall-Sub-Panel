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
