package httpserver

import (
	"crypto/sha256"
	"encoding/hex"
	"log/slog"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/vinzenzs/kazper/internal/auth"
	"github.com/vinzenzs/kazper/internal/idempotency"
)

// requestLogger logs one JSON line per request with request_id, client_id,
// status, latency, route, and a hashed idempotency key (never the raw key).
// Responses with status ≥ 500 are logged at Error level so a 5xx is loud and
// correlatable by request_id (which the request-id middleware set upstream).
func requestLogger(logger *slog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()
		latency := time.Since(start)

		status := c.Writer.Status()
		attrs := []any{
			"request_id", c.GetString("request_id"),
			"method", c.Request.Method,
			"path", c.Request.URL.Path,
			"route", c.FullPath(),
			"status", status,
			"latency_ms", latency.Milliseconds(),
			"client_id", string(auth.ClientFromContext(c)),
		}
		if k := c.GetHeader(idempotency.HeaderName); k != "" {
			attrs = append(attrs, "idempotency_key_sha256", hashKey(k))
		}
		if status >= 500 {
			logger.Error("request", attrs...)
			return
		}
		logger.Info("request", attrs...)
	}
}

func hashKey(k string) string {
	sum := sha256.Sum256([]byte(k))
	return hex.EncodeToString(sum[:8])
}
