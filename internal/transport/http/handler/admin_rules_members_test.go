package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	yamladapter "github.com/KazuhaHub/passwall-sub-panel/internal/adapters/yaml"
	"github.com/KazuhaHub/passwall-sub-panel/internal/domain"
)

type staticRuleNodes struct{ nodes []*domain.Node }

func (s staticRuleNodes) List(context.Context) ([]*domain.Node, error) { return s.nodes, nil }

func TestAdminRuleSetsSavePersistsMembersAndInvalidatesRenderCache(t *testing.T) {
	gin.SetMode(gin.TestMode)
	repo, err := yamladapter.NewRuleSetRepo(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	invalidations := 0
	h := NewAdminRuleSetsHandler(repo, staticRuleNodes{nodes: []*domain.Node{{ID: 42, DisplayName: "China", Enabled: true}}}, nil, func() { invalidations++ }, t.TempDir())

	body := ruleSetDTO{
		Slug: "custom", Name: "Custom", Enabled: true, Content: "- MATCH,🇨🇳 中国大陆",
		ProxyGroupMembers: map[string][]domain.ProxyGroupMember{
			"🇨🇳 中国大陆": {{Kind: "node", NodeID: 42}, {Kind: "builtin", Value: "DIRECT"}, {Kind: "node_set", Value: "remaining"}},
		},
	}
	w := performRuleSave(t, h, body)
	if w.Code != http.StatusNoContent {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
	if invalidations != 1 {
		t.Fatalf("invalidations=%d", invalidations)
	}
	got, err := repo.GetBySlug(context.Background(), "custom")
	if err != nil {
		t.Fatal(err)
	}
	if members := got.ProxyGroupMembers["🇨🇳 中国大陆"]; len(members) != 3 || members[0].NodeID != 42 {
		t.Fatalf("members=%#v", members)
	}
}

func TestAdminRuleSetsSaveRejectsMemberCycleWithoutInvalidating(t *testing.T) {
	repo, err := yamladapter.NewRuleSetRepo(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	invalidations := 0
	h := NewAdminRuleSetsHandler(repo, staticRuleNodes{}, nil, func() { invalidations++ }, t.TempDir())
	body := ruleSetDTO{
		Slug: "bad", Name: "Bad", Enabled: true, Content: "- DOMAIN,a,A\n- MATCH,B",
		ProxyGroupMembers: map[string][]domain.ProxyGroupMember{
			"A": {{Kind: "proxy_group", Value: "B"}},
			"B": {{Kind: "proxy_group", Value: "A"}},
		},
	}
	w := performRuleSave(t, h, body)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
	if invalidations != 0 {
		t.Fatalf("invalidations=%d", invalidations)
	}
}

func performRuleSave(t *testing.T, h *AdminRuleSetsHandler, body ruleSetDTO) *httptest.ResponseRecorder {
	t.Helper()
	raw, err := json.Marshal(body)
	if err != nil {
		t.Fatal(err)
	}
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPut, "/api/admin/rules/"+body.Slug, bytes.NewReader(raw))
	c.Request.Header.Set("Content-Type", "application/json")
	h.Save(c)
	c.Writer.WriteHeaderNow()
	return w
}
