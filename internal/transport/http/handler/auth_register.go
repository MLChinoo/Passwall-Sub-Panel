package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/KazuhaHub/passwall-sub-panel/internal/service/registration"
)

// AuthRegisterHandler exposes the public self-registration endpoints. Both sit
// behind the login rate limiter (wired in the router).
type AuthRegisterHandler struct {
	reg *registration.Service
}

func NewAuthRegisterHandler(reg *registration.Service) *AuthRegisterHandler {
	return &AuthRegisterHandler{reg: reg}
}

type registerRequest struct {
	Email       string `json:"email" binding:"required"`
	Password    string `json:"password" binding:"required"`
	DisplayName string `json:"display_name"`
}

// Register creates a new local account. On success returns whether the account
// still needs email verification. Maps validation / disabled / taken-email to
// the right status (taken-email IS revealed — expected signup UX).
func (h *AuthRegisterHandler) Register(c *gin.Context) {
	var req registerRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	res, err := h.reg.Register(c.Request.Context(), registration.RegisterInput{
		Email:       req.Email,
		Password:    req.Password,
		DisplayName: req.DisplayName,
	})
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true, "requires_verification": res.RequiresVerification})
}

type verifyEmailRequest struct {
	Token string `json:"token"`
	Ident string `json:"ident"`
	Code  string `json:"code"`
}

// VerifyEmail confirms the email and activates the account. 401 on a bad/expired
// token (deliberately generic).
func (h *AuthRegisterHandler) VerifyEmail(c *gin.Context) {
	var req verifyEmailRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := h.reg.Verify(c.Request.Context(), registration.VerifyInput{
		Token: req.Token,
		Ident: req.Ident,
		Code:  req.Code,
	}); err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}
