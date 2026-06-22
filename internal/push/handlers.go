package push

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/vinzenzs/kazper/internal/auth"
)

// Handlers wires the device push-token endpoints. They are restricted to the
// `mobile` identity and work whether or not the FCM sender is configured, so a
// device can register ahead of push being enabled.
type Handlers struct {
	svc *Service
}

func NewHandlers(svc *Service) *Handlers {
	return &Handlers{svc: svc}
}

func (h *Handlers) Register(rg *gin.RouterGroup) {
	rg.POST("/push/tokens", h.register)
	rg.DELETE("/push/tokens", h.remove)
}

// requireMobile enforces the mobile-only guard; device tokens are not an agent
// or garmin concern.
func (h *Handlers) requireMobile(c *gin.Context) bool {
	if auth.ClientFromContext(c) != auth.ClientMobile {
		c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "forbidden"})
		return false
	}
	return true
}

type tokenRequest struct {
	Token    string `json:"token"`
	Platform string `json:"platform,omitempty"`
}

// register godoc
// @Summary      Register a device push token (mobile identity only)
// @Description  Upserts the device's FCM registration token so the backend can deliver notifications (today: Garmin relogin). Idempotent by token. Restricted to the `mobile` identity. Accepts and stores the token even when push is unconfigured, so enabling FCM later delivers without re-pairing.
// @Tags         push
// @Accept       json
// @Produce      json
// @Param        body  body  tokenRequest  true  "FCM registration token"
// @Success      201  {object}  PushToken
// @Failure      400  {object}  map[string]string  "invalid_json | token_required"
// @Failure      403  {object}  map[string]string  "forbidden"
// @Security     BearerAuth
// @Router       /push/tokens [post]
func (h *Handlers) register(c *gin.Context) {
	if !h.requireMobile(c) {
		return
	}
	var req tokenRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_json"})
		return
	}
	if req.Token == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "token_required"})
		return
	}
	tok, err := h.svc.RegisterToken(c.Request.Context(), req.Token, req.Platform)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "register_failed"})
		return
	}
	c.JSON(http.StatusCreated, tok)
}

// remove godoc
// @Summary      Remove a device push token (mobile identity only)
// @Description  Drops the supplied device token (e.g. on sign-out). Restricted to the `mobile` identity. Removing an unknown token is a no-op.
// @Tags         push
// @Accept       json
// @Produce      json
// @Param        body  body  tokenRequest  true  "FCM registration token"
// @Success      204  "no content"
// @Failure      400  {object}  map[string]string  "invalid_json | token_required"
// @Failure      403  {object}  map[string]string  "forbidden"
// @Security     BearerAuth
// @Router       /push/tokens [delete]
func (h *Handlers) remove(c *gin.Context) {
	if !h.requireMobile(c) {
		return
	}
	var req tokenRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_json"})
		return
	}
	if req.Token == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "token_required"})
		return
	}
	if err := h.svc.RemoveToken(c.Request.Context(), req.Token); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "remove_failed"})
		return
	}
	c.Status(http.StatusNoContent)
}
