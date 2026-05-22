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
// from the local snapshot (zero 3X-UI calls). It is false only for nodes never
// captured — freshly imported, or a pre-v4 row before reconcile backfills it —
// which fall back to a one-shot live fetch.
func nodeHasLocalConfig(n *domain.Node) bool {
	return n != nil && n.ConfigSyncedAt != nil
}

// inboundForNodeRender returns the node's local config snapshot, or live-fetches
// it when the node hasn't been captured yet (transition window). The sing-box
// and URI-list paths call this per node; the mihomo path (buildProxies) batches
// its fallback fetch across panels, so it inlines the same decision instead.
func (s *Service) inboundForNodeRender(ctx context.Context, n *domain.Node) (*ports.Inbound, error) {
	if nodeHasLocalConfig(n) {
		return inboundFromNode(n), nil
	}
	return s.fetchInbound(ctx, n)
}
