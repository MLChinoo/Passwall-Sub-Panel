package render

import (
	"encoding/base64"
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

// TestBuildSS2022URI pins the SIP022 share-link shape: the 2022-blake3-*
// userinfo is the literal "method:serverPSK:userPSK" with the PSK base64
// specials (+ / =) percent-encoded — NOT the whole userinfo base64-wrapped
// (the SIP002 trick, which sing-box / shadowsocks-rust reject for 2022).
func TestBuildSS2022URI(t *testing.T) {
	got := buildSS2022URI("HK-1", "node.example.com", 8388,
		"2022-blake3-aes-256-gcm", "ab+cd/ef=", "12/34+56=")

	want := "ss://2022-blake3-aes-256-gcm:ab%2Bcd%2Fef%3D:12%2F34%2B56%3D@node.example.com:8388#HK-1"
	if got != want {
		t.Fatalf("ss2022 uri mismatch:\n got  %s\n want %s", got, want)
	}
	// Method must be visible in cleartext (i.e. the userinfo is not
	// base64-wrapped). A wrapped form would hide the method string.
	if !strings.Contains(got, "2022-blake3-aes-256-gcm:") {
		t.Fatalf("method should appear in cleartext, userinfo looks base64-wrapped: %s", got)
	}
}

// TestBuildSSURI keeps the legacy (non-2022) SIP002 contract intact: the
// userinfo IS base64url-encoded (no padding) as base64(method:password).
func TestBuildSSURI(t *testing.T) {
	got := buildSSURI("JP-1", "1.2.3.4", 8388, "aes-256-gcm", "p@ss")
	// base64url(no pad) of "aes-256-gcm:p@ss"
	want := "ss://YWVzLTI1Ni1nY206cEBzcw@1.2.3.4:8388#JP-1"
	if got != want {
		t.Fatalf("ss uri mismatch:\n got  %s\n want %s", got, want)
	}
}

// TestBuildVLESSURI_FlowVerbatim mirrors TestEmitVLESS_FlowVerbatim for the
// URI output: empty flow → no flow query param; explicit flow → emitted.
func TestBuildVLESSURI_FlowVerbatim(t *testing.T) {
	reality := xuiStreamSettings{Network: "tcp", Security: "reality", RealitySettings: &xuiRealitySettings{}}

	got := buildVLESSURI("n", "1.2.3.4", 443, "uuid-1", reality, "")
	if strings.Contains(got, "flow=") {
		t.Fatalf("empty flow must not be defaulted: %s", got)
	}

	got = buildVLESSURI("n", "1.2.3.4", 443, "uuid-1", reality, "xtls-rprx-vision")
	mustContain(t, got, "flow=xtls-rprx-vision")
}

func TestBuildSUIModernURIs(t *testing.T) {
	stream := xuiStreamSettings{Security: "tls", TLSSettings: &xuiTLSSettings{
		ServerName: "sni.example.com", ALPN: []string{"h3"}, AllowInsecure: true,
	}}
	stream.TLSSettings.Settings.Fingerprint = "chrome"
	anytls := buildAnyTLSURI("Any TLS", "edge.example.com", 443, "uuid-1", stream)
	if !strings.HasPrefix(anytls, "anytls://uuid-1@edge.example.com:443?") {
		t.Fatalf("AnyTLS URI = %s", anytls)
	}
	mustContain(t, anytls, "sni=sni.example.com")
	mustContain(t, anytls, "fp=chrome")

	tuic := buildTUICURI("TUIC", "edge.example.com", 443, "uuid-1", xuiInboundSettings{CongestionControl: "bbr"}, stream)
	if !strings.HasPrefix(tuic, "tuic://uuid-1:uuid-1@edge.example.com:443?") {
		t.Fatalf("TUIC URI = %s", tuic)
	}
	mustContain(t, tuic, "congestion_control=bbr")

	naive := buildNaiveURI("Naive", "edge.example.com", 443, "actual@psp.local", "uuid-1", stream)
	prefix := "http2://"
	encoded := strings.TrimPrefix(strings.SplitN(naive, "?", 2)[0], prefix)
	decoded, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		t.Fatalf("decode Naive userinfo: %v (%s)", err, naive)
	}
	if string(decoded) != "actual@psp.local:uuid-1@edge.example.com:443" {
		t.Fatalf("Naive authority = %q", decoded)
	}
	mustContain(t, naive, "peer=sni.example.com")
}

func mustContain(t *testing.T, s, sub string) {
	t.Helper()
	if !strings.Contains(s, sub) {
		t.Fatalf("expected %q in %q", sub, s)
	}
}
