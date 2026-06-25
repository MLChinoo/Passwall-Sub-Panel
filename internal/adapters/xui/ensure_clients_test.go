package xui

import (
	"encoding/json"
	"testing"
)

// TestEnsureClientsArray locks the fix for the live-observed bug where a
// recreated/created SHADOWSOCKS inbound rejected every client: 3X-UI's
// POST /clients/add appends to settings.clients in place, and when the field
// is absent (PSP stores its snapshot client-less) the append blanks out —
// HTTP 200, empty body, no client created. VLESS survived only because 3X-UI
// re-adds clients:[] itself on inbound creation; SS does not. The adapter must
// guarantee the array exists at inbound-creation time.
func TestEnsureClientsArray(t *testing.T) {
	hasEmptyClients := func(s string) bool {
		var m map[string]json.RawMessage
		if json.Unmarshal([]byte(s), &m) != nil {
			return false
		}
		c, ok := m["clients"]
		if !ok {
			return false
		}
		var arr []json.RawMessage
		return json.Unmarshal(c, &arr) == nil && len(arr) == 0
	}

	t.Run("ss settings without clients gets an empty array", func(t *testing.T) {
		in := `{"method":"2022-blake3-aes-256-gcm","password":"abc","network":"tcp,udp"}`
		out := ensureClientsArray(in)
		if !hasEmptyClients(out) {
			t.Fatalf("want clients:[] injected, got %s", out)
		}
		// protocol fields must survive
		var m map[string]any
		_ = json.Unmarshal([]byte(out), &m)
		if m["method"] != "2022-blake3-aes-256-gcm" || m["password"] != "abc" {
			t.Fatalf("protocol fields lost: %s", out)
		}
	})

	t.Run("settings with a populated clients array is left untouched", func(t *testing.T) {
		in := `{"clients":[{"email":"u1@x"}],"method":"aes-256-gcm"}`
		out := ensureClientsArray(in)
		var m struct {
			Clients []map[string]any `json:"clients"`
		}
		if err := json.Unmarshal([]byte(out), &m); err != nil || len(m.Clients) != 1 {
			t.Fatalf("must not drop existing clients, got %s", out)
		}
	})

	t.Run("vless settings with empty clients stays valid", func(t *testing.T) {
		in := `{"clients":[],"decryption":"none","fallbacks":[]}`
		if out := ensureClientsArray(in); !hasEmptyClients(out) {
			t.Fatalf("want clients:[] preserved, got %s", out)
		}
	})

	t.Run("blank settings becomes an object with an empty clients array", func(t *testing.T) {
		if out := ensureClientsArray("   "); !hasEmptyClients(out) {
			t.Fatalf("blank should yield clients:[], got %s", out)
		}
	})

	t.Run("non-object/malformed settings is returned verbatim", func(t *testing.T) {
		if out := ensureClientsArray("not json"); out != "not json" {
			t.Fatalf("malformed must pass through, got %s", out)
		}
	})
}
