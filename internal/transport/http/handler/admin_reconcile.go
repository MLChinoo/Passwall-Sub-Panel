package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/KazuhaHub/passwall-sub-panel/internal/service/reconcile"
)

type AdminReconcileHandler struct {
	recon *reconcile.Service
}

func NewAdminReconcileHandler(recon *reconcile.Service) *AdminReconcileHandler {
	return &AdminReconcileHandler{recon: recon}
}

// Run performs ONLY the lightweight per-node ownership reconcile, so the admin
// "Reconcile" button stays fast. It deliberately does NOT run the shared-client
// heal/merge/orphan-reconcile (ResyncMembership over every user) — that is heavy (a
// per-panel client list + provision per user, scaling with the user count) and runs
// on its own cadence in the reconcile loop (runReconcileLoop → HealSharedClients,
// every cron_reconcile minutes) plus once at boot. Folding it into this handler made
// every click take seconds.
func (h *AdminReconcileHandler) Run(c *gin.Context) {
	report, err := h.recon.RunOnce(c.Request.Context(), reconcile.LevelFull)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, report)
}
