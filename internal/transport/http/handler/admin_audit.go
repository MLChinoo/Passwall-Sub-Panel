package handler

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/KazuhaHub/passwall-sub-panel/internal/domain"
	"github.com/KazuhaHub/passwall-sub-panel/internal/ports"
	"github.com/KazuhaHub/passwall-sub-panel/internal/service/geo"
)

// AdminAuditHandler exposes /api/admin/audit — paginated audit log
// retrieval with optional actor / action / time-range filters.
type AdminAuditHandler struct {
	repo ports.AuditRepo
	geo  *geo.Service // nil-tolerant: nil/disabled → no region field
}

func NewAdminAuditHandler(repo ports.AuditRepo, geoSvc *geo.Service) *AdminAuditHandler {
	return &AdminAuditHandler{repo: repo, geo: geoSvc}
}

// auditView is an AuditEntry plus its resolved IP region (omitted when geo is
// disabled or the IP isn't resolved yet — the cache fills on later views).
type auditView struct {
	*domain.AuditEntry
	Region *domain.GeoLocation `json:"region,omitempty"`
}

func (h *AdminAuditHandler) List(c *gin.Context) {
	p := parsePagination(c)
	filter := ports.AuditFilter{
		Pagination: p,
		Actor:      c.Query("actor"),
		Action:     c.Query("action"),
		Search:     firstNonEmpty(p.Keyword, c.Query("search")),
	}
	if v := c.Query("since"); v != "" {
		if t, err := time.Parse(time.RFC3339, v); err == nil {
			filter.Since = &t
		}
	}
	if v := c.Query("until"); v != "" {
		if t, err := time.Parse(time.RFC3339, v); err == nil {
			filter.Until = &t
		}
	}
	items, total, err := h.repo.List(c.Request.Context(), filter)
	if err != nil {
		respondError(c, err)
		return
	}
	ips := make([]string, 0, len(items))
	for _, it := range items {
		ips = append(ips, it.IP)
	}
	regions := map[string]domain.GeoLocation{}
	if h.geo != nil {
		regions = h.geo.Lookup(c.Request.Context(), ips)
	}
	views := make([]auditView, len(items))
	for i, it := range items {
		views[i] = auditView{AuditEntry: it}
		if loc, ok := regions[it.IP]; ok {
			locCopy := loc
			views[i].Region = &locCopy
		}
	}
	c.JSON(http.StatusOK, pagedEnvelope(views, total, p))
}

func (h *AdminAuditHandler) Clear(c *gin.Context) {
	if err := h.repo.Clear(c.Request.Context()); err != nil {
		respondError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}
