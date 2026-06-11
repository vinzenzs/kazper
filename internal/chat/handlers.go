package chat

import (
	"context"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

// Handlers wires the chat service to the POST /chat route. svc may be nil when
// the server starts without an Anthropic API key — the handler then returns 503
// chat_unavailable, mirroring how meals/from_photo handles a missing vision key.
type Handlers struct {
	svc *Service
}

func NewHandlers(svc *Service) *Handlers {
	return &Handlers{svc: svc}
}

// Register mounts POST /chat onto rg.
func (h *Handlers) Register(rg *gin.RouterGroup) {
	rg.POST("/chat", h.chat)
}

// chat godoc
// @Summary      Stream a nutrition-planning chat turn
// @Description  Runs a server-side Anthropic agent loop scoped to meal planning and streams the result as Server-Sent Events. The request body is the full client-held transcript ({messages:[{role,content}]}); the server holds no conversation state. The response is text/event-stream with four event types — `text` (assistant delta), `tool` (name+status+summary), `done` (final message, stop_reason, usage), and `error` (typed code). Tools are dispatched as loopback REST calls under the caller's bearer token. Returns 503 chat_unavailable when ANTHROPIC_API_KEY is unset, and 400 for an empty transcript or a client-supplied system message.
// @Tags         chat
// @Accept       json
// @Produce      text/event-stream
// @Param        body  body  ChatRequest  true  "Client-held transcript"
// @Success      200   {string}  string  "SSE stream"
// @Failure      400   {object}  map[string]string  "invalid_json | empty_transcript | system_role_not_allowed | invalid_role"
// @Failure      503   {object}  map[string]string  "chat_unavailable"
// @Security     BearerAuth
// @Router       /chat [post]
func (h *Handlers) chat(c *gin.Context) {
	// 503 before any stream when the key is unset.
	if h.svc == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "chat_unavailable"})
		return
	}

	var req ChatRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_json"})
		return
	}
	if code := validateTranscript(req.Messages); code != "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": code})
		return
	}

	bearer := extractBearer(c.GetHeader("Authorization"))

	// Per-request timeout independent of the client connection's context.
	ctx, cancel := context.WithTimeout(c.Request.Context(), h.svc.cfg.RequestTimeout)
	defer cancel()

	sse, ok := newSSEWriter(c.Writer)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "streaming_unsupported"})
		return
	}
	h.svc.stream(ctx, sse, req.Messages, bearer)
}

// validateTranscript enforces the request-shape rules: non-empty, roles limited
// to user/assistant, and no client-supplied system message (the system prompt
// is server-owned and not overridable).
func validateTranscript(msgs []InboundMessage) string {
	if len(msgs) == 0 {
		return "empty_transcript"
	}
	for _, m := range msgs {
		switch m.Role {
		case "system":
			return "system_role_not_allowed"
		case "user", "assistant":
			// ok
		default:
			return "invalid_role"
		}
	}
	return ""
}

func extractBearer(authHeader string) string {
	const prefix = "Bearer "
	if strings.HasPrefix(authHeader, prefix) {
		return strings.TrimSpace(authHeader[len(prefix):])
	}
	return strings.TrimSpace(authHeader)
}
