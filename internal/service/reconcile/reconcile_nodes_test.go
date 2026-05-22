package reconcile

import (
	"context"
	"testing"
	"time"

	"github.com/KazuhaHub/passwall-sub-panel/internal/domain"
	"github.com/KazuhaHub/passwall-sub-panel/internal/ports"
)

// ---- minimal fakes (embed the interface, override only what checkNodes uses) ----

type recNodeRepo struct {
	ports.NodeRepo
	nodes   []*domain.Node
	updates []*domain.Node
}

func (r *recNodeRepo) List(context.Context) ([]*domain.Node, error) { return r.nodes, nil }
func (r *recNodeRepo) Update(_ context.Context, n *domain.Node) error {
	cp := *n
	r.updates = append(r.updates, &cp)
	return nil
}

type recPool struct{ c ports.XUIClient }

func (p recPool) Get(int64) (ports.XUIClient, error) { return p.c, nil }
func (recPool) List() []*domain.XUIPanel             { return nil }
func (recPool) Add(*domain.XUIPanel) error           { return nil }
func (recPool) Remove(int64) error                   { return nil }

type recClient struct {
	ports.XUIClient
	inbounds []ports.Inbound
	getResp  *ports.Inbound
	updated  []ports.InboundSpec
}

func (c *recClient) ListInbounds(context.Context) ([]ports.Inbound, error) { return c.inbounds, nil }
func (c *recClient) GetInbound(_ context.Context, id int) (*ports.Inbound, error) {
	if c.getResp != nil {
		return c.getResp, nil
	}
	for i := range c.inbounds {
		if c.inbounds[i].ID == id {
			return &c.inbounds[i], nil
		}
	}
	return nil, domain.ErrNotFound
}
func (c *recClient) UpdateInbound(_ context.Context, _ int, spec ports.InboundSpec) error {
	c.updated = append(c.updated, spec)
	return nil
}

// Node with no captured config + a live inbound → reconcile should pull the
// config into the node (backfill) and NOT push anything.
func TestCheckNodes_BackfillsMissingConfig(t *testing.T) {
	node := &domain.Node{ID: 1, PanelID: 1, InboundID: 3, Enabled: true} // ConfigSyncedAt nil
	client := &recClient{inbounds: []ports.Inbound{{
		ID: 3, Protocol: "vless", Port: 443,
		StreamSettings: `{"network":"tcp","security":"reality"}`,
		Settings:       `{"decryption":"none","clients":[{"id":"x"}]}`,
	}}}
	repo := &recNodeRepo{nodes: []*domain.Node{node}}
	svc := &Service{nodes: repo, pool: recPool{c: client}}

	report := &Report{}
	svc.checkNodes(context.Background(), report)

	if len(repo.updates) != 1 {
		t.Fatalf("want 1 node update (backfill), got %d", len(repo.updates))
	}
	got := repo.updates[0]
	if got.ConfigSyncedAt == nil || got.StreamSettings != `{"network":"tcp","security":"reality"}` {
		t.Fatalf("config not captured into node: %+v", got)
	}
	if len(client.updated) != 0 {
		t.Fatalf("backfill must not push to 3X-UI, got %d pushes", len(client.updated))
	}
	if report.Fixed != 1 {
		t.Fatalf("want report.Fixed=1, got %d", report.Fixed)
	}
}

// Node with a captured snapshot that differs from the live inbound → reconcile
// pushes PSP's config back, then re-captures the live config.
func TestCheckNodes_DriftPushed(t *testing.T) {
	now := time.Now()
	node := &domain.Node{
		ID: 1, PanelID: 1, InboundID: 3, Enabled: true,
		Protocol:        "vless",
		Port:            443,
		StreamSettings:  `{"network":"ws","security":"tls"}`, // PSP's truth
		InboundSettings: `{"decryption":"none"}`,
		ConfigSyncedAt:  &now,
	}
	live := ports.Inbound{
		ID: 3, Protocol: "vless", Port: 443,
		StreamSettings: `{"network":"tcp","security":"none"}`, // drifted on 3X-UI
		Settings:       `{"decryption":"none","clients":[{"id":"x"}]}`,
	}
	client := &recClient{inbounds: []ports.Inbound{live}, getResp: &live}
	repo := &recNodeRepo{nodes: []*domain.Node{node}}
	svc := &Service{nodes: repo, pool: recPool{c: client}}

	report := &Report{}
	svc.checkNodes(context.Background(), report)

	if len(client.updated) != 1 {
		t.Fatalf("want 1 push to 3X-UI on drift, got %d", len(client.updated))
	}
	if client.updated[0].StreamSettings != `{"network":"ws","security":"tls"}` {
		t.Fatalf("push must carry PSP's config, got %s", client.updated[0].StreamSettings)
	}
	if report.Fixed != 1 {
		t.Fatalf("want report.Fixed=1, got %d", report.Fixed)
	}
}

// Node whose snapshot already matches the live inbound (modulo clients[] and
// key ordering) → no push, no node write.
func TestCheckNodes_InSync_NoOp(t *testing.T) {
	now := time.Now()
	node := &domain.Node{
		ID: 1, PanelID: 1, InboundID: 3, Enabled: true,
		Protocol:        "vless",
		Port:            443,
		StreamSettings:  `{"security":"tls","network":"ws"}`, // key order differs only
		InboundSettings: `{"decryption":"none"}`,
		ConfigSyncedAt:  &now,
	}
	live := ports.Inbound{
		ID: 3, Protocol: "vless", Port: 443,
		StreamSettings: `{"network":"ws","security":"tls"}`,
		Settings:       `{"decryption":"none","clients":[{"id":"x"}]}`,
	}
	client := &recClient{inbounds: []ports.Inbound{live}}
	repo := &recNodeRepo{nodes: []*domain.Node{node}}
	svc := &Service{nodes: repo, pool: recPool{c: client}}

	report := &Report{}
	svc.checkNodes(context.Background(), report)

	if len(client.updated) != 0 || len(repo.updates) != 0 {
		t.Fatalf("in-sync node must be a no-op: pushes=%d updates=%d", len(client.updated), len(repo.updates))
	}
	if report.Fixed != 0 {
		t.Fatalf("want report.Fixed=0, got %d", report.Fixed)
	}
}
