package sync

import (
	"context"
	"testing"

	"github.com/KazuhaHub/passwall-sub-panel/internal/domain"
	"github.com/KazuhaHub/passwall-sub-panel/internal/ports"
)

func TestDelOwnedClientDeletesSSClientWithEmptyIDByEmail(t *testing.T) {
	xui := &fakeXUIClient{clients: []ports.ClientDetail{{Email: "u1@example.test"}}}
	own := newFakeOwnership("u1@example.test", "")
	s := New(&fakePool{xui: xui}, own)

	if err := s.DelOwnedClient(context.Background(), 1, 100, "u1@example.test"); err != nil {
		t.Fatal(err)
	}
	if xui.deletedByEmail != "u1@example.test" {
		t.Fatalf("deletedByEmail = %q", xui.deletedByEmail)
	}
	if !own.removed {
		t.Fatalf("ownership was not removed")
	}
}

func TestDelOwnedClientUsesCurrentClientIDInsteadOfStaleOwnershipUUID(t *testing.T) {
	xui := &fakeXUIClient{clients: []ports.ClientDetail{{ID: "current-id", Email: "u1@example.test"}}}
	own := newFakeOwnership("u1@example.test", "stale-id")
	s := New(&fakePool{xui: xui}, own)

	if err := s.DelOwnedClient(context.Background(), 1, 100, "u1@example.test"); err != nil {
		t.Fatal(err)
	}
	if xui.deletedID != "current-id" {
		t.Fatalf("deletedID = %q", xui.deletedID)
	}
	if !own.removed {
		t.Fatalf("ownership was not removed")
	}
}

type fakePool struct {
	xui ports.XUIClient
}

func (p *fakePool) Get(panelID int64) (ports.XUIClient, error) { return p.xui, nil }
func (p *fakePool) List() []*domain.XUIPanel                   { return nil }
func (p *fakePool) Add(panel *domain.XUIPanel) error           { return nil }
func (p *fakePool) Remove(panelID int64) error                 { return nil }

type fakeOwnership struct {
	entry   *domain.XUIClientEntry
	removed bool
}

func newFakeOwnership(email, uuid string) *fakeOwnership {
	return &fakeOwnership{entry: &domain.XUIClientEntry{
		UserID:      1,
		PanelID:     1,
		InboundID:   100,
		ClientEmail: email,
		ClientUUID:  uuid,
	}}
}

func (r *fakeOwnership) Add(ctx context.Context, e *domain.XUIClientEntry) error { return nil }
func (r *fakeOwnership) Remove(ctx context.Context, id int64) error              { return nil }
func (r *fakeOwnership) RemoveByMatch(ctx context.Context, panelID int64, inboundID int, email string) error {
	r.removed = true
	return nil
}
func (r *fakeOwnership) GetByMatch(ctx context.Context, panelID int64, inboundID int, email string) (*domain.XUIClientEntry, error) {
	return r.entry, nil
}
func (r *fakeOwnership) ListByUser(ctx context.Context, userID int64) ([]*domain.XUIClientEntry, error) {
	return nil, nil
}
func (r *fakeOwnership) ListByInbound(ctx context.Context, panelID int64, inboundID int) ([]*domain.XUIClientEntry, error) {
	return nil, nil
}
func (r *fakeOwnership) Exists(ctx context.Context, panelID int64, inboundID int, email string) (bool, error) {
	return true, nil
}
func (r *fakeOwnership) UpdateUUID(ctx context.Context, panelID int64, inboundID int, email, newUUID string) error {
	return nil
}
func (r *fakeOwnership) UpdateCounters(ctx context.Context, e *domain.XUIClientEntry) error {
	return nil
}

type fakeXUIClient struct {
	ports.XUIClient
	clients        []ports.ClientDetail
	deletedID      string
	deletedByEmail string
}

func (c *fakeXUIClient) GetInboundClients(ctx context.Context, inboundID int) ([]ports.ClientDetail, error) {
	return c.clients, nil
}
func (c *fakeXUIClient) DelClient(ctx context.Context, inboundID int, clientUUID string) error {
	c.deletedID = clientUUID
	return nil
}
func (c *fakeXUIClient) DelClientByEmail(ctx context.Context, inboundID int, email string) error {
	c.deletedByEmail = email
	return nil
}

var _ ports.OwnershipRepo = (*fakeOwnership)(nil)
var _ ports.XUIPool = (*fakePool)(nil)
var _ ports.XUIClient = (*fakeXUIClient)(nil)
