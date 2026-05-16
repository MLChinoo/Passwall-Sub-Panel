package render

import (
	"strings"
	"testing"
)

// TestBuildHysteria2URI pins the Hysteria 2 subscription URI shape that
// V2rayN / NekoBox / etc. consume. The wider format is documented at
// https://v2.hysteria.network/docs/developers/URI-Scheme/.
//
// Fields exercised:
//   - mandatory password (userinfo)
//   - host:port authority
//   - sni query (TLS server-name override)
//   - obfs + obfs-password
//   - insecure flag (1 when allowInsecure)
//   - alpn list (comma-separated)
//   - fragment = display name (URL-escaped)
func TestBuildHysteria2URI(t *testing.T) {
	got := buildHysteria2URI("US-1", "node.example.com", 8443, "secret-pwd", hysteria2Opts{
		SNI:          "node.example.com",
		ObfsType:     "salamander",
		ObfsPassword: "obfs-secret",
		ALPN:         []string{"h3"},
		Insecure:     false,
	})
	if !strings.HasPrefix(got, "hysteria2://secret-pwd@node.example.com:8443/?") {
		t.Fatalf("wrong prefix: %s", got)
	}
	mustContain(t, got, "sni=node.example.com")
	mustContain(t, got, "obfs=salamander")
	mustContain(t, got, "obfs-password=obfs-secret")
	mustContain(t, got, "alpn=h3")
	mustContain(t, got, "insecure=0")
	if !strings.HasSuffix(got, "#US-1") {
		t.Fatalf("wrong fragment: %s", got)
	}
}

// TestBuildHysteria2URI_NoObfs_Insecure covers the minimum-viable path:
// no obfs, allowInsecure=true. Obfs query keys MUST be absent when not
// configured (clients treat an empty obfs as "salamander with empty
// password" which breaks the connection).
func TestBuildHysteria2URI_NoObfs_Insecure(t *testing.T) {
	got := buildHysteria2URI("Test", "1.2.3.4", 443, "pwd", hysteria2Opts{
		Insecure: true,
	})
	mustContain(t, got, "insecure=1")
	if strings.Contains(got, "obfs=") {
		t.Fatalf("obfs key should be absent when ObfsType is empty: %s", got)
	}
}

func mustContain(t *testing.T, s, sub string) {
	t.Helper()
	if !strings.Contains(s, sub) {
		t.Fatalf("expected %q in %q", sub, s)
	}
}
