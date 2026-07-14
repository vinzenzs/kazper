package httpserver

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/vinzenzs/kazper/internal/reqid"
)

// hasExemptPrefix reports whether path starts with any of the given prefixes.
func hasExemptPrefix(path string, prefixes []string) bool {
	for _, p := range prefixes {
		if strings.HasPrefix(path, p) {
			return true
		}
	}
	return false
}

// requestIDMiddleware honors an inbound X-Request-ID or generates one, stores it
// on the request context (so the chat loopback dispatcher can forward it) and
// the Gin context (so the logger can read it), and echoes it on the response.
func requestIDMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.GetHeader(reqid.HeaderName)
		if id == "" {
			id = reqid.Generate()
		}
		c.Request = c.Request.WithContext(reqid.NewContext(c.Request.Context(), id))
		c.Writer.Header().Set(reqid.HeaderName, id)
		c.Set("request_id", id)
		c.Next()
	}
}

// requestTimeout imposes a per-request deadline via context. Handlers propagate
// c.Request.Context() into pgx, so a blocked query is abandoned at the deadline;
// if the deadline expires before anything is written, the response is a
// structured 504. Streaming/long routes are exempted by path prefix (they own
// their own budgets). This composes with the existing error shape — unlike
// http.TimeoutHandler, which buffers responses and breaks SSE.
func requestTimeout(d time.Duration, exempt []string) gin.HandlerFunc {
	return func(c *gin.Context) {
		if d <= 0 || hasExemptPrefix(c.Request.URL.Path, exempt) {
			c.Next()
			return
		}
		ctx, cancel := context.WithTimeout(c.Request.Context(), d)
		defer cancel()
		c.Request = c.Request.WithContext(ctx)
		c.Next()
		if errors.Is(ctx.Err(), context.DeadlineExceeded) && !c.Writer.Written() {
			c.AbortWithStatusJSON(http.StatusGatewayTimeout, gin.H{"error": "request_timeout"})
		}
	}
}

// bodyLimit rejects request bodies larger than maxBytes with a structured 413.
// It reads the (bounded) body up front and replaces it with an in-memory reader
// so downstream binds and the idempotency middleware see the same bytes. Routes
// that carry their own caps (meal photo, Garmin proxy uploads) are exempted so
// their larger limits govern.
func bodyLimit(maxBytes int64, exempt []string) gin.HandlerFunc {
	return func(c *gin.Context) {
		if maxBytes <= 0 || c.Request.Body == nil || c.Request.Body == http.NoBody ||
			hasExemptPrefix(c.Request.URL.Path, exempt) {
			c.Next()
			return
		}
		limited := http.MaxBytesReader(c.Writer, c.Request.Body, maxBytes)
		buf, err := io.ReadAll(limited)
		if err != nil {
			var mbe *http.MaxBytesError
			if errors.As(err, &mbe) {
				c.AbortWithStatusJSON(http.StatusRequestEntityTooLarge, gin.H{"error": "body_too_large"})
				return
			}
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": "invalid_request_body"})
			return
		}
		c.Request.Body = io.NopCloser(bytes.NewReader(buf))
		c.Request.ContentLength = int64(len(buf))
		c.Next()
	}
}
