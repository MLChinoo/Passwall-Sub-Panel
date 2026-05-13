package handler

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/KazuhaHub/passwall-sub-panel/internal/domain"
	"github.com/KazuhaHub/passwall-sub-panel/internal/service/auth"
	"github.com/KazuhaHub/passwall-sub-panel/internal/service/user"
)

// AuthOIDCHandler exposes /api/auth/oidc/{login,callback}.
//
// State and nonce are kept in short-lived HttpOnly cookies (5 minutes)
// and validated on the callback to defeat CSRF / replay. Once the IdP
// returns a verified ID token, the panel upserts a domain.User and
// hands back JWT cookies the same way SAML does.
type AuthOIDCHandler struct {
	oidc *auth.OIDCService
	auth *auth.Service
	user *user.Service
}

const (
	cookieOIDCState = "psp_oidc_state"
	cookieOIDCNonce = "psp_oidc_nonce"
	cookieOIDCRet   = "psp_oidc_return"
	oidcCookieTTL   = 300 // seconds
)

func NewAuthOIDCHandler(oidcSvc *auth.OIDCService, authSvc *auth.Service, userSvc *user.Service) *AuthOIDCHandler {
	return &AuthOIDCHandler{oidc: oidcSvc, auth: authSvc, user: userSvc}
}

func (h *AuthOIDCHandler) Login(c *gin.Context) {
	if !h.oidc.Enabled() {
		c.JSON(http.StatusNotFound, gin.H{"error": "oidc not enabled"})
		return
	}
	state, err := auth.RandomState()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	nonce, err := auth.RandomState()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	url, err := h.oidc.AuthCodeURL(state, nonce)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	returnTo := c.Query("return_to")
	if returnTo == "" {
		returnTo = "/user/me"
	}
	c.SetSameSite(http.SameSiteLaxMode)
	c.SetCookie(cookieOIDCState, state, oidcCookieTTL, "/", "", false, true)
	c.SetCookie(cookieOIDCNonce, nonce, oidcCookieTTL, "/", "", false, true)
	c.SetCookie(cookieOIDCRet, returnTo, oidcCookieTTL, "/", "", false, true)
	c.Redirect(http.StatusFound, url)
}

func (h *AuthOIDCHandler) Callback(c *gin.Context) {
	if !h.oidc.Enabled() {
		c.JSON(http.StatusNotFound, gin.H{"error": "oidc not enabled"})
		return
	}
	if errParam := c.Query("error"); errParam != "" {
		desc := c.Query("error_description")
		c.JSON(http.StatusUnauthorized, gin.H{"error": errParam, "description": desc})
		return
	}
	state := c.Query("state")
	wantState, _ := c.Cookie(cookieOIDCState)
	if state == "" || state != wantState {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "oauth2 state mismatch"})
		return
	}
	nonce, _ := c.Cookie(cookieOIDCNonce)
	code := c.Query("code")
	if code == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "missing authorization code"})
		return
	}
	// Clear the one-time cookies regardless of outcome.
	c.SetCookie(cookieOIDCState, "", -1, "/", "", false, true)
	c.SetCookie(cookieOIDCNonce, "", -1, "/", "", false, true)

	assertion, err := h.oidc.Exchange(c.Request.Context(), code, nonce)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
		return
	}

	isAdmin := h.oidc.IsAdmin(assertion.Groups)

	upn := assertion.Username
	u, err := h.user.EnsureSSO(c.Request.Context(), user.EnsureSSOInput{
		UPN:         upn,
		Email:       assertion.Email,
		DisplayName: assertion.DisplayName,
		Groups:      assertion.Groups,
		IsAdmin:     isAdmin,
	})
	if errors.Is(err, domain.ErrSSONoAccount) {
		c.Redirect(http.StatusFound, "/sso-no-account")
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if !u.Enabled {
		msg := "account disabled"
		if u.AutoDisabledReason == domain.DisabledPendingApproval {
			msg = "account pending admin approval"
		}
		c.JSON(http.StatusForbidden, gin.H{"error": msg})
		return
	}
	access, refresh, err := h.auth.IssueTokens(u)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	secure := false
	c.SetSameSite(http.SameSiteLaxMode)
	c.SetCookie(CookieAccessToken, access, int(h.auth.AccessTTL().Seconds()), "/", "", secure, true)
	c.SetCookie(CookieRefreshToken, refresh, int(h.auth.RefreshTTL().Seconds()), "/", "", secure, true)

	returnTo, _ := c.Cookie(cookieOIDCRet)
	c.SetCookie(cookieOIDCRet, "", -1, "/", "", false, true)
	if returnTo == "" {
		returnTo = "/user/me"
	}
	c.Redirect(http.StatusFound, "/sso-callback?next="+returnTo)
}
