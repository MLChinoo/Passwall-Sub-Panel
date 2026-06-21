package xui

import (
	"context"
	"crypto/tls"
	"net/http"
	"os"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/KazuhaHub/passwall-sub-panel/internal/ports"
)

// TestLive_MultiInboundClientSurface exercises the v3.9.0 attach/detach +
// multi-inbound add against a REAL 3X-UI panel. It is gated on env vars and
// skips by default (no secrets in the repo), mirroring the sqlstore live-DB
// tests' PSP_TEST_DB_* convention. Run with:
//
//	PSP_LIVE_XUI_URL='https://host:port/secretpath' \
//	PSP_LIVE_XUI_TOKEN='<api-token>' \
//	  go test ./internal/adapters/xui/ -run TestLive_MultiInboundClientSurface -v
//
// It creates one client on the first two inbounds, verifies the attachment set
// via GetClient().InboundIDs, detaches/re-attaches one, and always deletes the
// test client on the way out.
func TestLive_MultiInboundClientSurface(t *testing.T) {
	base := os.Getenv("PSP_LIVE_XUI_URL")
	token := os.Getenv("PSP_LIVE_XUI_TOKEN")
	if base == "" || token == "" {
		t.Skip("set PSP_LIVE_XUI_URL and PSP_LIVE_XUI_TOKEN to run the live 3X-UI smoke test")
	}

	// Construct the Client directly (in-package) with a PERMISSIVE http client
	// rather than via New(): New() installs safehttp.BlockNonPublicDial (SSRF
	// guard) + standard TLS verification. When this smoke test is run from a dev
	// box sitting behind a fake-IP proxy/TUN (clash/mihomo/sing-box map the
	// panel host into 198.18.0.0/15), the guard correctly refuses the
	// special-use address and the proxy may also MITM the cert — both are local
	// artifacts, not the behaviour we're testing. Bearer-token mode needs only
	// baseURL + apiToken.
	c := &Client{
		baseURL:  strings.TrimRight(base, "/"),
		apiToken: token,
		http: &http.Client{
			Timeout:   30 * time.Second,
			Transport: &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: true}}, //nolint:gosec // local smoke test only
		},
	}
	ctx := context.Background()

	inbounds, err := c.ListInbounds(ctx)
	if err != nil {
		t.Fatalf("ListInbounds: %v", err)
	}
	if len(inbounds) < 2 {
		t.Skipf("need >=2 inbounds for the multi-inbound test, panel has %d", len(inbounds))
	}
	a, b := inbounds[0].ID, inbounds[1].ID

	const email = "psp-livetest@psp.local"
	// Pre-clean any leftover from a previous aborted run, then guarantee teardown.
	_ = c.DelClientByEmail(ctx, a, email)
	t.Cleanup(func() { _ = c.DelClientByEmail(ctx, a, email) })

	// 1. Create one client attached to BOTH inbounds in a single call.
	if err := c.AddClientToInbounds(ctx, []int{a, b}, ports.ClientSpec{Email: email, Enable: true}); err != nil {
		t.Fatalf("AddClientToInbounds: %v", err)
	}
	assertAttached(t, c, ctx, email, a, b)

	// 2. Detach from b → only a remains.
	if err := c.DetachClient(ctx, email, []int{b}); err != nil {
		t.Fatalf("DetachClient: %v", err)
	}
	assertAttached(t, c, ctx, email, a)

	// 3. Re-attach b → both again.
	if err := c.AttachClient(ctx, email, []int{b}); err != nil {
		t.Fatalf("AttachClient: %v", err)
	}
	assertAttached(t, c, ctx, email, a, b)
}

// TestLive_SharedClientMigrationFlow mirrors what the v3.9.0 migration does to a
// real panel: create the shared client with the actual SILENT spec (id=uuid,
// password=uuid, auth=uuid — what buildSharedClientSpec produces for the default
// class) across two inbounds, confirm the attach, then delete it (the per-node
// cleanup). Verifies the real migration spec is accepted + fully removable.
func TestLive_SharedClientMigrationFlow(t *testing.T) {
	base := os.Getenv("PSP_LIVE_XUI_URL")
	token := os.Getenv("PSP_LIVE_XUI_TOKEN")
	if base == "" || token == "" {
		t.Skip("set PSP_LIVE_XUI_URL and PSP_LIVE_XUI_TOKEN to run the live 3X-UI smoke test")
	}
	c := &Client{
		baseURL:  strings.TrimRight(base, "/"),
		apiToken: token,
		http: &http.Client{
			Timeout:   30 * time.Second,
			Transport: &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: true}}, //nolint:gosec // local smoke test only
		},
	}
	ctx := context.Background()
	inbounds, err := c.ListInbounds(ctx)
	if err != nil {
		t.Fatalf("ListInbounds: %v", err)
	}
	if len(inbounds) < 2 {
		t.Skipf("need >=2 inbounds, panel has %d", len(inbounds))
	}
	a, b := inbounds[0].ID, inbounds[1].ID

	const email = "psp-migtest@psp.local"
	const uuid = "11111111-2222-3333-4444-555555555555"
	_ = c.DelClientByEmail(ctx, a, email)
	t.Cleanup(func() { _ = c.DelClientByEmail(ctx, a, email) })

	// Create with the silent migration spec (all per-protocol fields populated).
	spec := ports.ClientSpec{Email: email, Enable: true, ID: uuid, Password: uuid, Auth: uuid}
	if err := c.AddClientToInbounds(ctx, []int{a, b}, spec); err != nil {
		t.Fatalf("AddClientToInbounds (silent spec): %v", err)
	}
	assertAttached(t, c, ctx, email, a, b)

	// Delete (the migration's legacy cleanup) → the client is fully gone.
	if err := c.DelClientByEmail(ctx, a, email); err != nil {
		t.Fatalf("DelClientByEmail: %v", err)
	}
	if cd, err := c.GetClient(ctx, email); err != nil {
		t.Fatalf("GetClient after delete: %v", err)
	} else if cd != nil {
		t.Fatalf("client must be gone after DelClientByEmail, got inboundIds=%v", cd.InboundIDs)
	}
}

func assertAttached(t *testing.T, c *Client, ctx context.Context, email string, want ...int) {
	t.Helper()
	cd, err := c.GetClient(ctx, email)
	if err != nil {
		t.Fatalf("GetClient: %v", err)
	}
	if cd == nil {
		t.Fatalf("GetClient(%s) = nil, want a client", email)
	}
	got := append([]int(nil), cd.InboundIDs...)
	sort.Ints(got)
	sort.Ints(want)
	if len(got) != len(want) {
		t.Fatalf("inboundIds = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("inboundIds = %v, want %v", got, want)
		}
	}
}
