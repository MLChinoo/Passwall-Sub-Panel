// Package sharedclient is the v3.9.0 cutover Stage-1b reconcile service: it
// CREATES the shared client in 3X-UI for a psp_client and confirms the result
// before marking each attachment provisioned. It is the only writer of the
// per-(client,node) Provisioned flag, which render/traffic later consult.
//
// It is additive/dormant: the shared client is created enable=true with no
// expiry/quota (the full lifecycle is wired in Stage 1c, BEFORE any render flip),
// and nothing renders the shared client yet — so a created-but-unmanaged client
// carries no traffic and is harmless. It coexists with the legacy per-node
// clients until Stage 4 removes them.
package sharedclient

import (
	"context"
	"fmt"

	"github.com/KazuhaHub/passwall-sub-panel/internal/domain"
	"github.com/KazuhaHub/passwall-sub-panel/internal/pkg/log"
	"github.com/KazuhaHub/passwall-sub-panel/internal/ports"
)

type Service struct {
	clients ports.PSPClientRepo
	pool    ports.XUIPool
	nodes   ports.NodeRepo
}

func New(clients ports.PSPClientRepo, pool ports.XUIPool, nodes ports.NodeRepo) *Service {
	return &Service{clients: clients, pool: pool, nodes: nodes}
}

// ProvisionResult summarizes one provisioning pass.
type ProvisionResult struct {
	Created     bool // a 3X-UI client create/attach was issued
	Provisioned int  // attachments confirmed present in 3X-UI and marked
	Skipped     int  // attachments whose node could not be resolved
}

// buildSharedClientSpec maps a psp_client to the 3X-UI client spec, carrying the
// STORED credentials (not derived). One client object holds every protocol's
// field — id (VLESS/VMess), password (Trojan/SS/SS-2022), auth (Hysteria2 = the
// UUID, matching what render emits) — and 3X-UI projects only the relevant field
// into each inbound. Flow is the partition's single effective flow. Enable is
// true; expiry/quota are left 0 — Stage 1c owns the lifecycle.
func buildSharedClientSpec(c *domain.PSPClient, flow string) ports.ClientSpec {
	return ports.ClientSpec{
		ID:       c.UUID,
		Email:    c.Email,
		Enable:   true,
		Flow:     flow,
		Password: c.Password,
		Auth:     c.UUID,
	}
}

// ProvisionClient creates/attaches the shared client for one psp_client across
// all its attached inbounds in a single AddClientToInbounds (one Xray restart),
// reads it back, and marks Provisioned only the attachments 3X-UI confirms.
// Idempotent: AddClient on an existing email re-attaches (3X-UI keys by email),
// so a re-run heals a partial attach.
func (s *Service) ProvisionClient(ctx context.Context, c *domain.PSPClient) (ProvisionResult, error) {
	var res ProvisionResult
	if c == nil {
		return res, nil
	}
	atts, err := s.clients.ListInbounds(ctx, c.ID)
	if err != nil {
		return res, fmt.Errorf("list attachments: %w", err)
	}
	if len(atts) == 0 {
		return res, nil
	}
	flow := atts[0].FlowOverride // uniform across a partition (the key's flow)

	inboundIDs := make([]int, 0, len(atts))
	nodeByInbound := make(map[int]int64, len(atts))
	for _, a := range atts {
		n, err := s.nodes.GetByID(ctx, a.NodeID)
		if err != nil || n == nil {
			log.Warn("sharedclient: resolve node", "client_id", c.ID, "node_id", a.NodeID, "err", err)
			res.Skipped++
			continue
		}
		if n.PanelID != c.PanelID {
			log.Warn("sharedclient: node panel mismatch", "client_id", c.ID, "node_id", a.NodeID,
				"node_panel", n.PanelID, "client_panel", c.PanelID)
			res.Skipped++
			continue
		}
		inboundIDs = append(inboundIDs, n.InboundID)
		nodeByInbound[n.InboundID] = a.NodeID
	}
	if len(inboundIDs) == 0 {
		return res, nil
	}

	cli, err := s.pool.Get(c.PanelID)
	if err != nil {
		return res, fmt.Errorf("xui pool get %d: %w", c.PanelID, err)
	}
	if err := cli.AddClientToInbounds(ctx, inboundIDs, buildSharedClientSpec(c, flow)); err != nil {
		return res, fmt.Errorf("create shared client %s: %w", c.Email, err)
	}
	res.Created = true

	// Read-back: only mark Provisioned the inbounds 3X-UI actually confirms the
	// client is attached to (the gate render/traffic trust — never "we asked").
	detail, err := cli.GetClient(ctx, c.Email)
	if err != nil {
		return res, fmt.Errorf("confirm shared client %s: %w", c.Email, err)
	}
	if detail == nil {
		return res, fmt.Errorf("shared client %s absent after create", c.Email)
	}
	confirmed := make(map[int]bool, len(detail.InboundIDs))
	for _, id := range detail.InboundIDs {
		confirmed[id] = true
	}
	for inb, nodeID := range nodeByInbound {
		if !confirmed[inb] {
			continue
		}
		if err := s.clients.MarkInboundProvisioned(ctx, c.ID, nodeID, true); err != nil {
			log.Warn("sharedclient: mark provisioned", "client_id", c.ID, "node_id", nodeID, "err", err)
			continue
		}
		res.Provisioned++
	}
	return res, nil
}

// ProvisionAll provisions every psp_client in the system — the operator-run
// Stage-1 pass that creates all shared clients in 3X-UI. Per-client failures are
// logged and counted; the first is returned but every client is attempted.
func (s *Service) ProvisionAll(ctx context.Context) (ProvisionResult, error) {
	all, err := s.clients.ListAll(ctx)
	if err != nil {
		return ProvisionResult{}, fmt.Errorf("list clients: %w", err)
	}
	var total ProvisionResult
	var firstErr error
	for _, c := range all {
		r, err := s.ProvisionClient(ctx, c)
		if err != nil {
			log.Warn("sharedclient: provision", "client_id", c.ID, "email", c.Email, "err", err)
			if firstErr == nil {
				firstErr = err
			}
		}
		if r.Created {
			total.Created = true
		}
		total.Provisioned += r.Provisioned
		total.Skipped += r.Skipped
	}
	log.Info("sharedclient: provision-all complete", "provisioned", total.Provisioned, "skipped", total.Skipped)
	return total, firstErr
}

// ProvisionUser provisions every shared client a user holds (across panels).
// Returns the first error but attempts all clients.
func (s *Service) ProvisionUser(ctx context.Context, userID int64) (ProvisionResult, error) {
	clients, err := s.clients.ListByUser(ctx, userID)
	if err != nil {
		return ProvisionResult{}, fmt.Errorf("list clients: %w", err)
	}
	var total ProvisionResult
	var firstErr error
	for _, c := range clients {
		r, err := s.ProvisionClient(ctx, c)
		if err != nil && firstErr == nil {
			firstErr = err
		}
		if r.Created {
			total.Created = true
		}
		total.Provisioned += r.Provisioned
		total.Skipped += r.Skipped
	}
	return total, firstErr
}
