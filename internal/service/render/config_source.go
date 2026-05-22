package render

import (
	"context"

	"github.com/KazuhaHub/passwall-sub-panel/internal/domain"
	"github.com/KazuhaHub/passwall-sub-panel/internal/ports"
	"github.com/KazuhaHub/passwall-sub-panel/internal/service/inboundcfg"
)

// inboundFromNode reconstructs a ports.Inbound from the node's locally stored
// config snapshot (v4: PSP is the source of truth for inbound config). The
// mapping lives in inboundcfg so the node service and reconcile share it.
func inboundFromNode(n *domain.Node) *ports.Inbound {
	return inboundcfg.InboundFromNode(n)
}

// nodeHasLocalConfig reports whether render can build this node's proxy block
// from the local snapshot (zero 3X-UI calls). False for:
//   - never captured (pre-v4 row before reconcile backfills it, or freshly
//     imported before the capture step ran) — ConfigSyncedAt is nil
//   - explicitly marked non-synced (future states like "broken" / "needs-attention"
//     that a writer wants to gate render off of) — state is non-empty and not
//     "synced". Today markSynced is the only writer and always sets "synced",
//     so this branch is only forward-compat insurance.
func nodeHasLocalConfig(n *domain.Node) bool {
	if n == nil || n.ConfigSyncedAt == nil {
		return false
	}
	switch n.ConfigSyncState {
	case "", "synced":
		return true
	default:
		return false
	}
}

// inboundForNodeRender returns the node's local config snapshot, or live-fetches
// it when the node hasn't been captured yet (transition window).
//
// All three production render paths (mihomo / sing-box / URI-list) now bucket
// un-captured nodes by panel and call prefetchInboundsForRender once per
// render — one ListInbounds per panel instead of one GetInbound per node.
// This helper survives for unit tests that need to exercise the decision
// in isolation; new render code should prefer the bulk path.
func (s *Service) inboundForNodeRender(ctx context.Context, n *domain.Node) (*ports.Inbound, error) {
	if nodeHasLocalConfig(n) {
		return inboundFromNode(n), nil
	}
	return s.fetchInbound(ctx, n)
}
