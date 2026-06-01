package handler

import (
	"github.com/gin-gonic/gin"

	"github.com/KazuhaHub/passwall-sub-panel/internal/domain"
	"github.com/KazuhaHub/passwall-sub-panel/internal/pkg/log"
	"github.com/KazuhaHub/passwall-sub-panel/internal/ports"
)

// recordAuthEvent writes one row to the authentication-event log. Best-effort:
// a logging failure must never block or fail the actual login. Synchronous on
// purpose — login is rate-limited and low-frequency, so the single INSERT is
// negligible, and inline keeps the event durable before the response is sent
// (so the request context is still alive). Call it BEFORE writing the response.
func recordAuthEvent(c *gin.Context, repo ports.AuthEventRepo, method domain.AuthMethod, outcome domain.AuthOutcome, userID int64, upn, reason string) {
	if repo == nil {
		return
	}
	e := &domain.AuthEvent{
		UserID:  userID,
		UPN:     upn,
		Method:  method,
		Outcome: outcome,
		Reason:  reason,
		IP:      c.ClientIP(),
		UA:      c.GetHeader("User-Agent"),
	}
	if err := repo.Insert(c.Request.Context(), e); err != nil {
		log.Warn("auth event insert failed", "method", method, "outcome", outcome, "upn", upn, "err", err)
	}
}
