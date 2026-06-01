package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/KazuhaHub/passwall-sub-panel/internal/service/geo"
)

// AdminGeoIPHandler exposes /api/admin/settings/geoip/* — the offline geo
// database status (available .mmdb files + which is active) and a manual
// "update now" trigger.
type AdminGeoIPHandler struct {
	geo *geo.Service // nil-tolerant
}

func NewAdminGeoIPHandler(geoSvc *geo.Service) *AdminGeoIPHandler {
	return &AdminGeoIPHandler{geo: geoSvc}
}

// Status lists the .mmdb files present, their type/granularity/build date, and
// which is active — so the admin UI can render the source dropdown + status.
func (h *AdminGeoIPHandler) Status(c *gin.Context) {
	if h.geo == nil {
		c.JSON(http.StatusOK, geo.Status{Available: []geo.DBStatus{}})
		return
	}
	c.JSON(http.StatusOK, h.geo.Status(c.Request.Context()))
}

// Update triggers an immediate download/refresh of the configured source's
// database (no user IPs involved — only a public DB is fetched).
func (h *AdminGeoIPHandler) Update(c *gin.Context) {
	if h.geo == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Geo service not available"})
		return
	}
	file, err := h.geo.Update(c.Request.Context())
	if err != nil {
		// Upstream/config failure (bad token, network, invalid file) — surface
		// the reason so the admin can fix it. 502: we depend on an external DB.
		c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"file": file})
}
