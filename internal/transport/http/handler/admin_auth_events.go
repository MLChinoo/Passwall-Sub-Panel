package handler

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/KazuhaHub/passwall-sub-panel/internal/domain"
	"github.com/KazuhaHub/passwall-sub-panel/internal/ports"
	"github.com/KazuhaHub/passwall-sub-panel/internal/service/geo"
)

// AdminAuthEventsHandler exposes /api/admin/auth-events — the first-class
// authentication-event log (logins across local / SAML / OIDC, success +
// failure) with optional method / outcome / user / time-range filters. IP
// region is resolved at view time (offline, same as the audit/sub-log views).
type AdminAuthEventsHandler struct {
	repo ports.AuthEventRepo
	geo  *geo.Service // nil-tolerant: nil/disabled → no region field
}

func NewAdminAuthEventsHandler(repo ports.AuthEventRepo, geoSvc *geo.Service) *AdminAuthEventsHandler {
	return &AdminAuthEventsHandler{repo: repo, geo: geoSvc}
}

// authEventView is an AuthEvent plus its resolved IP region (omitted when geo
// is disabled, the IP is private/unmapped, or no .mmdb is loaded).
type authEventView struct {
	*domain.AuthEvent
	Region *domain.GeoLocation `json:"region,omitempty"`
}

func (h *AdminAuthEventsHandler) List(c *gin.Context) {
	p := parsePagination(c)
	filter := ports.AuthEventFilter{
		Pagination: p,
		Method:     c.Query("method"),
		Outcome:    c.Query("outcome"),
		Search:     firstNonEmpty(p.Keyword, c.Query("search")),
	}
	if v := c.Query("user_id"); v != "" {
		if id, err := strconv.ParseInt(v, 10, 64); err == nil {
			filter.UserID = &id
		}
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
	views := make([]authEventView, len(items))
	for i, it := range items {
		views[i] = authEventView{AuthEvent: it}
		if loc, ok := regions[it.IP]; ok {
			locCopy := loc
			views[i].Region = &locCopy
		}
	}
	c.JSON(http.StatusOK, pagedEnvelope(views, total, p))
}
