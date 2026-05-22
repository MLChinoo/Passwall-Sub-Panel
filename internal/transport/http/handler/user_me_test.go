package handler

import (
	"testing"

	"github.com/KazuhaHub/passwall-sub-panel/internal/domain"
)

// TestCoarseNodeStatus pins the user-facing health bucketing. The security
// point: every distinct failure state (panel unreachable vs inbound missing vs
// inbound disabled) must collapse to the same "down" so the user-facing server
// status can't reveal WHERE in the stack a node failed — only that it's up,
// down, or not yet probed.
func TestCoarseNodeStatus(t *testing.T) {
	cases := []struct {
		in   domain.NodeHealthState
		want string
	}{
		{domain.NodeHealthOK, "ok"},
		{domain.NodeHealthPanelUnreachable, "down"},
		{domain.NodeHealthInboundMissing, "down"},
		{domain.NodeHealthInboundDisabled, "down"},
		{domain.NodeHealthUnknown, "unknown"},
		{domain.NodeHealthState("some-future-state"), "unknown"},
	}
	for _, tc := range cases {
		if got := coarseNodeStatus(tc.in); got != tc.want {
			t.Fatalf("coarseNodeStatus(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}
