package node

import (
	"context"
	"strings"
	"testing"

	"github.com/KazuhaHub/passwall-sub-panel/internal/domain"
	"github.com/KazuhaHub/passwall-sub-panel/internal/ports"
)

// UpdateInboundConfig write-through integration. The field-level mapping
// (clients[] stripping, snapshot capture) is unit-tested in the inboundcfg
// package; here we verify the service persists locally before pushing.

type captureNodeRepo struct {
	fakeNodeRepo
	node    *domain.Node
	updated *domain.Node
}

func (r *captureNodeRepo) GetByID(_ context.Context, _ int64) (*domain.Node, error) {
	if r.node == nil {
		return nil, domain.ErrNotFound
	}
	cp := *r.node
	return &cp, nil
}
func (r *captureNodeRepo) Update(_ context.Context, n *domain.Node) error {
	r.updated = n
	return nil
}

type stubXUIClient struct {
	ports.XUIClient
	updated *ports.InboundSpec
}

func (c *stubXUIClient) UpdateInbound(_ context.Context, _ int, spec ports.InboundSpec) error {
	c.updated = &spec
	return nil
}

type stubXUIPool struct {
	c   ports.XUIClient
	err error
}

func (p stubXUIPool) Get(int64) (ports.XUIClient, error) { return p.c, p.err }
func (stubXUIPool) List() []*domain.XUIPanel             { return nil }
func (stubXUIPool) Add(*domain.XUIPanel) error           { return nil }
func (stubXUIPool) Remove(int64) error                   { return nil }

func updateSpec() ports.InboundSpec {
	return ports.InboundSpec{
		Protocol:       "vless",
		Port:           443,
		Settings:       `{"decryption":"none","clients":[{"id":"x","email":"e"}]}`,
		StreamSettings: `{"network":"ws","security":"tls"}`,
	}
}

func TestUpdateInboundConfig_WriteThrough_PushOK(t *testing.T) {
	repo := &captureNodeRepo{node: &domain.Node{ID: 1, PanelID: 1, InboundID: 3}}
	client := &stubXUIClient{}
	svc := &Service{nodes: repo, pool: stubXUIPool{c: client}}

	if err := svc.UpdateInboundConfig(context.Background(), 1, updateSpec()); err != nil {
		t.Fatalf("UpdateInboundConfig = %v, want nil", err)
	}
	if repo.updated == nil {
		t.Fatalf("config not persisted locally (write-through missing)")
	}
	if repo.updated.StreamSettings != `{"network":"ws","security":"tls"}` {
		t.Fatalf("stream settings not stored: %+v", repo.updated)
	}
	if strings.Contains(repo.updated.InboundSettings, "clients") {
		t.Fatalf("stored settings must drop clients[]: %s", repo.updated.InboundSettings)
	}
	if repo.updated.ConfigSyncedAt == nil {
		t.Fatalf("ConfigSyncedAt should be set after write-through")
	}
	if client.updated == nil {
		t.Fatalf("config not pushed to 3X-UI")
	}
}

// Push fails (panel unreachable) but the local snapshot must still be written —
// local-first means render reflects the new config even while 3X-UI is down.
func TestUpdateInboundConfig_PushFails_StillStoredLocally(t *testing.T) {
	repo := &captureNodeRepo{node: &domain.Node{ID: 1, PanelID: 1, InboundID: 3}}
	svc := &Service{nodes: repo, pool: stubXUIPool{err: errPanelDown{}}}

	if err := svc.UpdateInboundConfig(context.Background(), 1, updateSpec()); err != nil {
		t.Fatalf("UpdateInboundConfig = %v, want nil (push failure is enqueued, not returned)", err)
	}
	if repo.updated == nil || repo.updated.StreamSettings == "" {
		t.Fatalf("config must be persisted locally even when the push fails")
	}
}

type errPanelDown struct{}

func (errPanelDown) Error() string { return "panel unreachable" }
