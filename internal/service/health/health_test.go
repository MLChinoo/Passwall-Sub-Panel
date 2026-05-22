package health

import (
	"context"
	"errors"
	"strconv"
	"testing"
	"time"

	"github.com/KazuhaHub/passwall-sub-panel/internal/domain"
	"github.com/KazuhaHub/passwall-sub-panel/internal/ports"
)

// --- pure logic: isUDPProtocol ----------------------------------------------

func TestIsUDPProtocol(t *testing.T) {
	for _, p := range []string{"hysteria2", "Hysteria2", "hy2", "hysteria"} {
		if !isUDPProtocol(p) {
			t.Fatalf("isUDPProtocol(%q) = false, want true", p)
		}
	}
	for _, p := range []string{"vless", "vmess", "trojan", "shadowsocks", ""} {
		if isUDPProtocol(p) {
			t.Fatalf("isUDPProtocol(%q) = true, want false", p)
		}
	}
}

// --- integration: CheckOnce orchestration ----------------------------------

type fakeNodeRepo struct {
	nodes   []*domain.Node
	updates []*domain.Node
}

func (r *fakeNodeRepo) List(ctx context.Context) ([]*domain.Node, error) {
	return r.nodes, nil
}
func (r *fakeNodeRepo) ListEnabled(ctx context.Context) ([]*domain.Node, error) {
	out := []*domain.Node{}
	for _, n := range r.nodes {
		if n.Enabled {
			out = append(out, n)
		}
	}
	return out, nil
}
func (r *fakeNodeRepo) GetByID(ctx context.Context, id int64) (*domain.Node, error) {
	for _, n := range r.nodes {
		if n.ID == id {
			return n, nil
		}
	}
	return nil, domain.ErrNotFound
}
func (r *fakeNodeRepo) GetByPanelInbound(ctx context.Context, panelID int64, inboundID int) (*domain.Node, error) {
	return nil, domain.ErrNotFound
}
func (r *fakeNodeRepo) Update(ctx context.Context, n *domain.Node) error {
	cp := *n
	r.updates = append(r.updates, &cp)
	return nil
}
func (r *fakeNodeRepo) UpdateHealth(ctx context.Context, n *domain.Node) error {
	cp := *n
	r.updates = append(r.updates, &cp)
	return nil
}
func (r *fakeNodeRepo) UpdateTrafficCounters(ctx context.Context, n *domain.Node) error { return nil }
func (r *fakeNodeRepo) UpdateInboundConfig(ctx context.Context, n *domain.Node) error   { return nil }
func (r *fakeNodeRepo) Create(ctx context.Context, n *domain.Node) error                       { return nil }
func (r *fakeNodeRepo) Delete(ctx context.Context, id int64) error                             { return nil }
func (r *fakeNodeRepo) BatchUpdateSortOrder(ctx context.Context, updates []ports.NodeSortUpdate) error {
	return nil
}

type fakeXUIClient struct {
	inbounds []ports.Inbound
	err      error
}

func (c *fakeXUIClient) ListInbounds(ctx context.Context) ([]ports.Inbound, error) {
	return c.inbounds, c.err
}
func (c *fakeXUIClient) GetInbound(ctx context.Context, id int) (*ports.Inbound, error) {
	return nil, domain.ErrNotFound
}
func (c *fakeXUIClient) AddInbound(ctx context.Context, spec ports.InboundSpec) (int, error) {
	return 0, nil
}
func (c *fakeXUIClient) UpdateInbound(ctx context.Context, id int, spec ports.InboundSpec) error {
	return nil
}
func (c *fakeXUIClient) DelInbound(ctx context.Context, id int) error                         { return nil }
func (c *fakeXUIClient) SetInboundEnable(ctx context.Context, id int, enable bool) error      { return nil }
func (c *fakeXUIClient) AddClient(ctx context.Context, id int, spec ports.ClientSpec) error   { return nil }
func (c *fakeXUIClient) UpdateClient(ctx context.Context, id int, uuid string, s ports.ClientSpec) error {
	return nil
}
func (c *fakeXUIClient) DelClient(ctx context.Context, id int, uuid string) error           { return nil }
func (c *fakeXUIClient) DelClientByEmail(ctx context.Context, id int, email string) error   { return nil }
func (c *fakeXUIClient) CopyClients(ctx context.Context, src, dst int, emails []string) error {
	return nil
}
func (c *fakeXUIClient) GetClientTraffic(ctx context.Context, email string) ([]ports.ClientTraffic, error) {
	return nil, nil
}
func (c *fakeXUIClient) GetInboundTraffics(ctx context.Context, id int) ([]ports.ClientTraffic, error) {
	return nil, nil
}
func (c *fakeXUIClient) ResetClientTraffic(ctx context.Context, id int, email string) error {
	return nil
}
func (c *fakeXUIClient) GetInboundClients(ctx context.Context, id int) ([]ports.ClientDetail, error) {
	return nil, nil
}

type fakePool struct {
	clients map[int64]ports.XUIClient
	err     map[int64]error
}

func (p *fakePool) Get(panelID int64) (ports.XUIClient, error) {
	if e, ok := p.err[panelID]; ok {
		return nil, e
	}
	c, ok := p.clients[panelID]
	if !ok {
		return nil, domain.ErrNotFound
	}
	return c, nil
}
func (p *fakePool) List() []*domain.XUIPanel       { return nil }
func (p *fakePool) Add(panel *domain.XUIPanel) error { return nil }
func (p *fakePool) Remove(panelID int64) error      { return nil }

// newWithProbe builds a Service with an injected probe that records the
// (network, host, port) it was called with and returns the given error.
func newWithProbe(repo ports.NodeRepo, pool ports.XUIPool, ret error) (*Service, *[]string) {
	calls := &[]string{}
	s := New(repo, pool)
	s.probe = func(_ context.Context, network, host string, port int) error {
		*calls = append(*calls, network+" "+host+":"+strconv.Itoa(port))
		return ret
	}
	return s, calls
}

func TestCheckOnce_PortOpen_Up(t *testing.T) {
	repo := &fakeNodeRepo{nodes: []*domain.Node{
		{ID: 1, PanelID: 10, InboundID: 1, Enabled: true, ServerAddress: "a.example"},
	}}
	pool := &fakePool{clients: map[int64]ports.XUIClient{
		10: &fakeXUIClient{inbounds: []ports.Inbound{{ID: 1, Enable: true, Port: 443, Protocol: "vless"}}},
	}}
	s, calls := newWithProbe(repo, pool, nil)
	if err := s.CheckOnce(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(repo.updates) != 1 || repo.updates[0].HealthState != domain.NodeHealthOK {
		t.Fatalf("want one OK update, got %+v", repo.updates)
	}
	if repo.updates[0].HealthCheckedAt == nil {
		t.Fatal("HealthCheckedAt must be stamped every pass")
	}
	if repo.updates[0].Port != 443 {
		t.Fatalf("port not cached: got %d, want 443", repo.updates[0].Port)
	}
	if len(*calls) != 1 || (*calls)[0] != "tcp a.example:443" {
		t.Fatalf("probe calls = %v, want one tcp a.example:443", *calls)
	}
}

func TestCheckOnce_PortClosed_Down(t *testing.T) {
	repo := &fakeNodeRepo{nodes: []*domain.Node{
		{ID: 1, PanelID: 10, InboundID: 1, Enabled: true, ServerAddress: "a.example"},
	}}
	pool := &fakePool{clients: map[int64]ports.XUIClient{
		10: &fakeXUIClient{inbounds: []ports.Inbound{{ID: 1, Enable: true, Port: 443, Protocol: "vless"}}},
	}}
	s, _ := newWithProbe(repo, pool, errors.New("connection refused"))
	if err := s.CheckOnce(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(repo.updates) != 1 || repo.updates[0].HealthState != domain.NodeHealthUnreachable {
		t.Fatalf("want one unreachable update, got %+v", repo.updates)
	}
}

func TestCheckOnce_UDPProtocolProbedWithUDP(t *testing.T) {
	repo := &fakeNodeRepo{nodes: []*domain.Node{
		{ID: 1, PanelID: 10, InboundID: 1, Enabled: true, ServerAddress: "h.example"},
	}}
	pool := &fakePool{clients: map[int64]ports.XUIClient{
		10: &fakeXUIClient{inbounds: []ports.Inbound{{ID: 1, Enable: true, Port: 8443, Protocol: "hysteria2"}}},
	}}
	s, calls := newWithProbe(repo, pool, nil)
	if err := s.CheckOnce(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(*calls) != 1 || (*calls)[0] != "udp h.example:8443" {
		t.Fatalf("hysteria2 must be probed over udp; calls = %v", *calls)
	}
}

func TestCheckOnce_PanelUnreachable_NoCachedPort(t *testing.T) {
	repo := &fakeNodeRepo{nodes: []*domain.Node{
		{ID: 1, PanelID: 10, InboundID: 1, Enabled: true, ServerAddress: "a.example"}, // Port 0 (never learned)
	}}
	pool := &fakePool{clients: map[int64]ports.XUIClient{
		10: &fakeXUIClient{err: errors.New("connection refused")},
	}}
	s, calls := newWithProbe(repo, pool, nil)
	if err := s.CheckOnce(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(*calls) != 0 {
		t.Fatalf("must not probe without a known port; calls = %v", *calls)
	}
	if len(repo.updates) != 1 || repo.updates[0].HealthState != domain.NodeHealthPanelUnreachable {
		t.Fatalf("want panel_unreachable, got %+v", repo.updates)
	}
}

func TestCheckOnce_PanelUnreachable_UsesCachedPort(t *testing.T) {
	// Panel API is down but we cached the port last time → still probe it, so a
	// node whose proxy port is reachable stays up even when 3X-UI's API isn't.
	repo := &fakeNodeRepo{nodes: []*domain.Node{
		{ID: 1, PanelID: 10, InboundID: 1, Enabled: true, ServerAddress: "a.example", Port: 443, Protocol: "vless"},
	}}
	pool := &fakePool{clients: map[int64]ports.XUIClient{
		10: &fakeXUIClient{err: errors.New("connection refused")},
	}}
	s, calls := newWithProbe(repo, pool, nil)
	if err := s.CheckOnce(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(*calls) != 1 || (*calls)[0] != "tcp a.example:443" {
		t.Fatalf("cached port should be probed; calls = %v", *calls)
	}
	if repo.updates[0].HealthState != domain.NodeHealthOK {
		t.Fatalf("want OK from cached-port probe, got %q", repo.updates[0].HealthState)
	}
}

func TestCheckOnce_InboundMissing(t *testing.T) {
	repo := &fakeNodeRepo{nodes: []*domain.Node{
		{ID: 1, PanelID: 10, InboundID: 9, Enabled: true, ServerAddress: "a.example"},
	}}
	pool := &fakePool{clients: map[int64]ports.XUIClient{
		10: &fakeXUIClient{inbounds: []ports.Inbound{{ID: 1, Enable: true, Port: 443}}},
	}}
	s, calls := newWithProbe(repo, pool, nil)
	if err := s.CheckOnce(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(*calls) != 0 {
		t.Fatalf("missing inbound (no port) must not be probed; calls = %v", *calls)
	}
	if repo.updates[0].HealthState != domain.NodeHealthInboundMissing {
		t.Fatalf("want inbound_missing, got %q", repo.updates[0].HealthState)
	}
}

func TestCheckOnce_DisabledNodesSkipped(t *testing.T) {
	repo := &fakeNodeRepo{nodes: []*domain.Node{
		{ID: 1, PanelID: 10, InboundID: 1, Enabled: false, HealthState: domain.NodeHealthOK},
	}}
	s, _ := newWithProbe(repo, &fakePool{}, nil)
	if err := s.CheckOnce(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(repo.updates) != 0 {
		t.Fatalf("disabled node was probed (%d updates); admin took it out of rotation deliberately", len(repo.updates))
	}
}

// Sanity: Loop respects ctx cancellation promptly.
func TestLoopReturnsOnCancel(t *testing.T) {
	repo := &fakeNodeRepo{}
	pool := &fakePool{}
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // already done before Loop starts
	done := make(chan struct{})
	go func() {
		New(repo, pool).Loop(ctx, 1*time.Hour)
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Loop did not return after ctx cancellation")
	}
}
