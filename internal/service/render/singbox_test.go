package render

import (
	"encoding/json"
	"reflect"
	"testing"
)

// TestBuildSingBoxHysteria2Outbound checks the sing-box outbound JSON
// shape per https://sing-box.sagernet.org/configuration/outbound/hysteria2/.
// Mandatory: type, server, server_port, password. Optional: obfs object,
// tls{enabled, server_name, alpn, insecure}.
func TestBuildSingBoxHysteria2Outbound(t *testing.T) {
	got := buildSingBoxHysteria2Outbound("hy2-us-1", "node.example.com", 8443, "secret-pwd", hysteria2Opts{
		SNI:          "node.example.com",
		ObfsType:     "salamander",
		ObfsPassword: "obfs-secret",
		ALPN:         []string{"h3"},
		Insecure:     false,
	})
	want := map[string]any{
		"tag":         "hy2-us-1",
		"type":        "hysteria2",
		"server":      "node.example.com",
		"server_port": 8443,
		"password":    "secret-pwd",
		"obfs": map[string]any{
			"type":     "salamander",
			"password": "obfs-secret",
		},
		"tls": map[string]any{
			"enabled":     true,
			"server_name": "node.example.com",
			"alpn":        []string{"h3"},
			"insecure":    false,
		},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %#v, want %#v", got, want)
	}
}

// TestBuildSingBoxHysteria2Outbound_NoObfs: when obfs isn't configured,
// the "obfs" key MUST be omitted (sing-box treats it as enabled-with-
// empty-password otherwise).
func TestBuildSingBoxHysteria2Outbound_NoObfs(t *testing.T) {
	got := buildSingBoxHysteria2Outbound("x", "1.2.3.4", 443, "p", hysteria2Opts{Insecure: true})
	if _, ok := got["obfs"]; ok {
		t.Fatalf("obfs key should be absent: %#v", got)
	}
	tls := got["tls"].(map[string]any)
	if tls["insecure"] != true {
		t.Fatalf("tls.insecure = %v, want true", tls["insecure"])
	}
}


func TestBuildSingBoxRouteRules(t *testing.T) {
	rules, final := buildSingBoxRouteRules(`
- DOMAIN-SUFFIX,example.com,🚀 节点选择
- DOMAIN,ads.example,REJECT
- IP-CIDR,1.1.1.1/32,🎯 全球直连,no-resolve
- GEOIP,CN,🇨🇳 中国大陆
- MATCH,🐟 漏网之鱼
`)
	if final != "🐟 漏网之鱼" {
		t.Fatalf("final = %q, want 漏网之鱼", final)
	}
	if len(rules) != 4 {
		t.Fatalf("rules len = %d, want 4: %#v", len(rules), rules)
	}
	if got := rules[0]["outbound"]; got != "🚀 节点选择" {
		t.Fatalf("domain suffix outbound = %q", got)
	}
	if got := rules[1]["outbound"]; got != "block" {
		t.Fatalf("reject outbound = %q", got)
	}
	if _, ok := rules[2]["ip_cidr"]; !ok {
		t.Fatalf("ip-cidr rule missing ip_cidr: %#v", rules[2])
	}
	if got := rules[3]["geoip"].([]string)[0]; got != "cn" {
		t.Fatalf("geoip = %q, want cn", got)
	}
}

func TestBuildSingBoxRouteRulesPersonalRulesFirst(t *testing.T) {
	personal := `
- DOMAIN-SUFFIX,example.com,💬 Ai平台
- MATCH,🎯 全球直连
`
	common := `
- DOMAIN-SUFFIX,example.com,🚀 节点选择
- MATCH,🐟 漏网之鱼
`
	rules, final := buildSingBoxRouteRules(personal, common)
	if final != "🎯 全球直连" {
		t.Fatalf("final = %q, want personal MATCH target", final)
	}
	if len(rules) != 1 {
		t.Fatalf("rules len = %d, want only personal rule before MATCH: %#v", len(rules), rules)
	}
	if got := rules[0]["outbound"]; got != "💬 Ai平台" {
		t.Fatalf("personal rule outbound = %q", got)
	}
}

func TestBuildSingBoxSelectorOutbounds(t *testing.T) {
	raw := `
- DOMAIN-SUFFIX,example.com,💬 Ai平台
- MATCH,🐟 漏网之鱼
`
	selectors := buildSingBoxSelectorOutbounds(raw, []string{"node-a", "node-b"}, nil)
	if len(selectors) != 3 {
		t.Fatalf("selectors len = %d, want 3: %#v", len(selectors), selectors)
	}
	if selectors[0]["tag"] != "🚀 节点选择" {
		t.Fatalf("first selector = %q", selectors[0]["tag"])
	}
	choices := selectors[1]["outbounds"].([]string)
	if len(choices) != 4 || choices[0] != "🚀 节点选择" || choices[1] != "node-a" || choices[3] != "direct" {
		t.Fatalf("ai selector choices = %#v", choices)
	}
}

func TestBuildSingBoxSelectorOutboundsUsesManualDisplayOrder(t *testing.T) {
	raw := `
- DOMAIN-SUFFIX,direct.example,🎯 全球直连
- DOMAIN-SUFFIX,ads.example,🛑 广告拦截
- DOMAIN-SUFFIX,ai.example,💬 Ai平台
- DOMAIN-SUFFIX,game.example,🎮 游戏平台
- MATCH,🐟 漏网之鱼
`
	selectors := buildSingBoxSelectorOutbounds(raw, []string{"node-a"}, []string{"🚀 节点选择", "💬 Ai平台", "🎮 游戏平台"})
	got := make([]string, len(selectors))
	for i, selector := range selectors {
		got[i] = selector["tag"].(string)
	}
	want := []string{"🚀 节点选择", "💬 Ai平台", "🎮 游戏平台", "🎯 全球直连", "🛑 广告拦截", "🐟 漏网之鱼"}
	if len(got) != len(want) {
		t.Fatalf("selectors len = %d, want %d: %#v", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("selector[%d] = %q, want %q; all=%#v", i, got[i], want[i], got)
		}
	}
}

func TestMarshalJSONBlockProducesValidReadableJSON(t *testing.T) {
	raw, err := marshalJSONBlock([]map[string]any{
		{"type": "selector", "tag": "🚀 节点选择", "outbounds": []string{"direct"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	var decoded []map[string]any
	if err := json.Unmarshal([]byte(raw), &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded[0]["tag"] != "🚀 节点选择" {
		t.Fatalf("decoded tag = %q", decoded[0]["tag"])
	}
}
