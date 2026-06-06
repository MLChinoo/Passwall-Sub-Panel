package handler

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

// Admin-side passkey management (v3.7.0). Listing is read-only metadata
// (id/name/timestamps, never the credential record) so it isn't gated beyond
// staff access — mirroring how Get exposes a redacted account. The destructive
// revoke paths reuse ensureOperatorAllowed so an operator can't strip an
// admin/operator account's passkeys, matching Reset2FA. Disabling the passkey
// feature does NOT remove already-enrolled credentials, so these endpoints stay
// useful (and available) even when new enrollment is off.

// ListUserPasskeys returns a target user's registered passkeys for the admin UI.
func (h *AdminUserHandler) ListUserPasskeys(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid id"})
		return
	}
	if h.passkey == nil {
		c.JSON(http.StatusOK, gin.H{"passkeys": []passkeyDTO{}})
		return
	}
	creds, err := h.passkey.List(c.Request.Context(), id)
	if err != nil {
		respondError(c, err)
		return
	}
	out := make([]passkeyDTO, 0, len(creds))
	for _, cr := range creds {
		out = append(out, passkeyDTO{ID: cr.ID, Name: cr.Name, CreatedAt: cr.CreatedAt, LastUsedAt: cr.LastUsedAt})
	}
	c.JSON(http.StatusOK, gin.H{"passkeys": out})
}

// RevokeUserPasskey removes one of a user's passkeys (lost/compromised device).
// The delete is scoped to (passkey id, user id) at the repo, so a mismatched
// passkey id can't reach another account's credential.
func (h *AdminUserHandler) RevokeUserPasskey(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid id"})
		return
	}
	pkID, err := strconv.ParseInt(c.Param("pkid"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid passkey id"})
		return
	}
	if !h.ensureOperatorAllowed(c, id) {
		return
	}
	if h.passkey == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Passkeys unavailable"})
		return
	}
	if err := h.passkey.Delete(c.Request.Context(), pkID, id); err != nil {
		respondError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

// RevokeAllUserPasskeys drops every passkey on the target account in one shot —
// the "lost all devices / compromised" break-glass, mirroring Reset2FA.
func (h *AdminUserHandler) RevokeAllUserPasskeys(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid id"})
		return
	}
	if !h.ensureOperatorAllowed(c, id) {
		return
	}
	if h.passkey == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Passkeys unavailable"})
		return
	}
	n, err := h.passkey.RevokeAll(c.Request.Context(), id)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"revoked": n})
}
