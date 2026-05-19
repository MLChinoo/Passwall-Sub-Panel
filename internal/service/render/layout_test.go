package render

import (
	"testing"

	"github.com/KazuhaHub/passwall-sub-panel/internal/domain"
)

// TestApplyLayout_SeparatorMergedBySortOrder documents that standalone
// separators (from the v3.0.0-beta.7 nodes_separator table) are merged
// into the node sequence by their shared SortOrder space, with the
// separator preferred above an equally-weighted node so admins can
// label a region group.
func TestApplyLayout_SeparatorMergedBySortOrder(t *testing.T) {
	nodes := []*domain.Node{
		{ID: 1, DisplayName: "TW Static", SortOrder: 10, Region: "TW"},
		{ID: 3, DisplayName: "TW Dynamic", SortOrder: 20, Region: "TW"},
	}
	seps := []*domain.SeparatorEntry{
		{ID: 2, DisplayName: "---- Taiwan HiNet ----", SortOrder: 5, Enabled: true, Mode: domain.SeparatorModeGlobal},
	}

	items := applyLayout(nodes, seps, domain.Layout{})
	if len(items) != 3 {
		t.Fatalf("got %d items, want 3", len(items))
	}
	if !items[0].isSeparator {
		t.Errorf("items[0].isSeparator = false, want true (separator at SortOrder 5)")
	}
	if items[0].name != "---- Taiwan HiNet ----" {
		t.Errorf("items[0].name = %q, want display_name verbatim", items[0].name)
	}
	if items[0].node != nil {
		t.Errorf("items[0].node should be nil for separator entries (got %+v)", items[0].node)
	}
	if items[1].isSeparator || items[1].node == nil || items[1].node.ID != 1 {
		t.Errorf("items[1] should wrap the real node id=1, got %+v", items[1])
	}
	if items[2].isSeparator || items[2].node == nil || items[2].node.ID != 3 {
		t.Errorf("items[2] should wrap the real node id=3, got %+v", items[2])
	}
}

// TestApplyLayout_NoSeparators is the baseline: real-only node list with
// nil separator slice should produce the same item count and ordering
// as before — guards against accidental nil-deref in the merged sort.
func TestApplyLayout_NoSeparators(t *testing.T) {
	n := &domain.Node{ID: 42, DisplayName: "legacy", SortOrder: 10}
	items := applyLayout([]*domain.Node{n}, nil, domain.Layout{})
	if len(items) != 1 {
		t.Fatalf("got %d items", len(items))
	}
	if items[0].isSeparator {
		t.Errorf("node should not render as separator")
	}
	if items[0].node == nil || items[0].node.ID != 42 {
		t.Errorf("node should wrap as real node, got %+v", items[0])
	}
}

// TestApplyLayout_SeparatorTieAboveNode covers the equal-SortOrder rule:
// separator and node share weight 10, separator sorts first so it labels
// the group below it.
func TestApplyLayout_SeparatorTieAboveNode(t *testing.T) {
	nodes := []*domain.Node{
		{ID: 1, DisplayName: "n1", SortOrder: 10},
	}
	seps := []*domain.SeparatorEntry{
		{ID: 2, DisplayName: "----", SortOrder: 10, Enabled: true, Mode: domain.SeparatorModeGlobal},
	}
	items := applyLayout(nodes, seps, domain.Layout{})
	if len(items) != 2 {
		t.Fatalf("got %d items, want 2", len(items))
	}
	if !items[0].isSeparator {
		t.Errorf("on tie, separator should sort before node")
	}
}
