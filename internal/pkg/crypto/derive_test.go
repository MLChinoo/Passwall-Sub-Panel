package crypto

import (
	"testing"

	"github.com/KazuhaHub/passwall-sub-panel/internal/domain"
)

// TestDetectProtocol_All covers every protocol family the panel renders
// subscriptions for. Keeps the mapping pinned so a renaming somewhere in
// 3X-UI's wire protocol can't silently change the dispatch result.
func TestDetectProtocol_All(t *testing.T) {
	cases := []struct {
		name, in, method string
		want             domain.Protocol
	}{
		{"vless", "vless", "", domain.ProtoVLESS},
		{"vmess", "vmess", "", domain.ProtoVMess},
		{"trojan", "trojan", "", domain.ProtoTrojan},
		{"ss-legacy", "shadowsocks", "aes-256-gcm", domain.ProtoSS},
		{"ss2022", "shadowsocks", "2022-blake3-aes-128-gcm", domain.ProtoSS2022},
		{"hysteria2", "hysteria2", "", domain.ProtoHysteria2},
		{"case-insensitive", "VLESS", "", domain.ProtoVLESS},
		{"unknown", "dokodemo-door", "", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := DetectProtocol(tc.in, tc.method)
			if got != tc.want {
				t.Fatalf("DetectProtocol(%q,%q) = %q, want %q", tc.in, tc.method, got, tc.want)
			}
		})
	}
}
