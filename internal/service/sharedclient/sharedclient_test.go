package sharedclient

import (
	"context"
	"testing"

	"github.com/KazuhaHub/passwall-sub-panel/internal/domain"
	"github.com/KazuhaHub/passwall-sub-panel/internal/ports"
)

// --- embedded-interface fakes: override only what ProvisionClient calls ---

type fakeClients struct {
	ports.PSPClientRepo
	attachments []domain.PSPClientInbound
	provisioned map[int64]bool // nodeID -> provisioned
}

func (f *fakeClients) ListInbounds(context.Context, int64) ([]domain.PSPClientInbound, error) {
	return f.attachments, nil
}
func (f *fakeClients) MarkInboundProvisioned(_ context.Context, _ int64, nodeID int64, p bool) error {
	if f.provisioned == nil {
		f.provisioned = map[int64]bool{}
	}
	f.provisioned[nodeID] = p
	return nil
}

type fakeNodes struct {
	ports.NodeRepo
	byID map[int64]*domain.Node
}

func (f fakeNodes) GetByID(_ context.Context, id int64) (*domain.Node, error) {
	if n, ok := f.byID[id]; ok {
		return n, nil
	}
	return nil, domain.ErrNotFound
}

type fakeXUI struct {
	ports.XUIClient
	addedInbounds []int
	addedSpec     ports.ClientSpec
	confirm       []int // inboundIDs GetClient reports the client attached to
}

func (c *fakeXUI) AddClientToInbounds(_ context.Context, inboundIDs []int, spec ports.ClientSpec) error {
	c.addedInbounds = append([]int(nil), inboundIDs...)
	c.addedSpec = spec
	return nil
}
func (c *fakeXUI) GetClient(context.Context, string) (*ports.ClientDetail, error) {
	return &ports.ClientDetail{InboundIDs: c.confirm}, nil
}

type fakePool struct {
	ports.XUIPool
	c ports.XUIClient
}

func (p fakePool) Get(int64) (ports.XUIClient, error) { return p.c, nil }

func TestProvisionClient_CreatesAndMarksConfirmed(t *testing.T) {
	clients := &fakeClients{attachments: []domain.PSPClientInbound{
		{ClientID: 1, NodeID: 11, FlowOverride: "xtls-rprx-vision"},
		{ClientID: 1, NodeID: 12, FlowOverride: "xtls-rprx-vision"},
	}}
	nodes := fakeNodes{byID: map[int64]*domain.Node{
		11: {ID: 11, PanelID: 10, InboundID: 101},
		12: {ID: 12, PanelID: 10, InboundID: 102},
	}}
	xui := &fakeXUI{confirm: []int{101, 102}} // 3X-UI confirms both
	svc := New(clients, fakePool{c: xui}, nodes)

	c := &domain.PSPClient{ID: 1, PanelID: 10, Email: "u1@psp.local", UUID: "uuid-x", Password: "pw-x"}
	res, err := svc.ProvisionClient(context.Background(), c)
	if err != nil {
		t.Fatal(err)
	}
	// One AddClientToInbounds over BOTH inbounds (single Xray restart).
	if len(xui.addedInbounds) != 2 || xui.addedInbounds[0] != 101 || xui.addedInbounds[1] != 102 {
		t.Fatalf("added inbounds = %v, want [101 102]", xui.addedInbounds)
	}
	// Spec carries stored creds + flow for multi-protocol projection.
	if xui.addedSpec.ID != "uuid-x" || xui.addedSpec.Password != "pw-x" || xui.addedSpec.Auth != "uuid-x" ||
		xui.addedSpec.Flow != "xtls-rprx-vision" || !xui.addedSpec.Enable {
		t.Fatalf("spec = %+v", xui.addedSpec)
	}
	if !res.Created || res.Provisioned != 2 {
		t.Fatalf("result = %+v, want created + 2 provisioned", res)
	}
	if !clients.provisioned[11] || !clients.provisioned[12] {
		t.Fatalf("both nodes should be marked provisioned: %v", clients.provisioned)
	}
}

func TestProvisionClient_MarksOnlyConfirmed(t *testing.T) {
	clients := &fakeClients{attachments: []domain.PSPClientInbound{
		{ClientID: 1, NodeID: 11},
		{ClientID: 1, NodeID: 12},
	}}
	nodes := fakeNodes{byID: map[int64]*domain.Node{
		11: {ID: 11, PanelID: 10, InboundID: 101},
		12: {ID: 12, PanelID: 10, InboundID: 102},
	}}
	// 3X-UI confirms only inbound 101 (102's attach silently failed).
	xui := &fakeXUI{confirm: []int{101}}
	svc := New(clients, fakePool{c: xui}, nodes)

	res, err := svc.ProvisionClient(context.Background(), &domain.PSPClient{ID: 1, PanelID: 10, Email: "u1@psp.local"})
	if err != nil {
		t.Fatal(err)
	}
	if res.Provisioned != 1 || !clients.provisioned[11] || clients.provisioned[12] {
		t.Fatalf("only node 11 should be provisioned (12 unconfirmed): res=%+v marks=%v", res, clients.provisioned)
	}
}

func TestProvisionClient_SkipsUnresolvableNode(t *testing.T) {
	clients := &fakeClients{attachments: []domain.PSPClientInbound{
		{ClientID: 1, NodeID: 11},
		{ClientID: 1, NodeID: 99}, // not in nodes repo
	}}
	nodes := fakeNodes{byID: map[int64]*domain.Node{11: {ID: 11, PanelID: 10, InboundID: 101}}}
	xui := &fakeXUI{confirm: []int{101}}
	svc := New(clients, fakePool{c: xui}, nodes)

	res, err := svc.ProvisionClient(context.Background(), &domain.PSPClient{ID: 1, PanelID: 10, Email: "u1@psp.local"})
	if err != nil {
		t.Fatal(err)
	}
	if res.Skipped != 1 || res.Provisioned != 1 {
		t.Fatalf("result = %+v, want 1 skipped + 1 provisioned", res)
	}
	if len(xui.addedInbounds) != 1 || xui.addedInbounds[0] != 101 {
		t.Fatalf("only the resolvable inbound should be added: %v", xui.addedInbounds)
	}
}
