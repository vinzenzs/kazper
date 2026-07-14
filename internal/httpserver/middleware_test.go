package httpserver

import (
	"bytes"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"

	"github.com/vinzenzs/kazper/internal/reqid"
)

func init() { gin.SetMode(gin.TestMode) }

// A non-exempt handler that blocks past the deadline (writing nothing) is cut
// off with a structured 504; an exempt prefix is never timed out.
func TestRequestTimeout(t *testing.T) {
	r := gin.New()
	g := r.Group("/api/v1")
	g.Use(requestTimeout(40*time.Millisecond, []string{"/api/v1/chat"}))
	g.GET("/slow", func(c *gin.Context) {
		<-c.Request.Context().Done() // respect the deadline, write nothing
	})
	g.GET("/chat/stream", func(c *gin.Context) {
		time.Sleep(90 * time.Millisecond)
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/v1/slow", nil))
	require.Equal(t, http.StatusGatewayTimeout, rec.Code)
	require.Contains(t, rec.Body.String(), "request_timeout")

	rec = httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/v1/chat/stream", nil))
	require.Equal(t, http.StatusOK, rec.Code, "exempt route must not be timed out")
}

// Oversized bodies are 413'd; bodies within the cap pass through intact; exempt
// routes keep their own (larger) caps.
func TestBodyLimit(t *testing.T) {
	r := gin.New()
	g := r.Group("/api/v1")
	g.Use(bodyLimit(10, []string{"/api/v1/meals/from_photo"}))
	echo := func(c *gin.Context) {
		b, _ := io.ReadAll(c.Request.Body)
		c.JSON(http.StatusOK, gin.H{"n": len(b)})
	}
	g.POST("/echo", echo)
	g.POST("/meals/from_photo", echo)

	// 11 bytes > 10 → 413.
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/v1/echo", strings.NewReader("12345678901")))
	require.Equal(t, http.StatusRequestEntityTooLarge, rec.Code)
	require.Contains(t, rec.Body.String(), "body_too_large")

	// Within the cap → passes through, body intact for the handler.
	rec = httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/v1/echo", strings.NewReader("hi")))
	require.Equal(t, http.StatusOK, rec.Code)
	require.Contains(t, rec.Body.String(), `"n":2`)

	// Exempt route accepts an over-cap body (its own limit governs).
	rec = httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/v1/meals/from_photo", strings.NewReader("12345678901")))
	require.Equal(t, http.StatusOK, rec.Code)
	require.Contains(t, rec.Body.String(), `"n":11`)
}

// The middleware generates and echoes an id, exposes it on the request context,
// and honors an inbound X-Request-ID.
func TestRequestID_GenerateEchoAndHonor(t *testing.T) {
	r := gin.New()
	g := r.Group("/api/v1")
	g.Use(requestIDMiddleware())
	var seen string
	g.GET("/probe", func(c *gin.Context) {
		seen = reqid.FromContext(c.Request.Context())
		c.Status(http.StatusOK)
	})

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/v1/probe", nil))
	gen := rec.Header().Get(reqid.HeaderName)
	require.NotEmpty(t, gen, "a request id must be generated")
	require.Equal(t, gen, seen, "the same id must be on the request context")

	rec = httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/probe", nil)
	req.Header.Set(reqid.HeaderName, "inbound-123")
	r.ServeHTTP(rec, req)
	require.Equal(t, "inbound-123", rec.Header().Get(reqid.HeaderName))
	require.Equal(t, "inbound-123", seen, "an inbound id must be honored, not replaced")
}

// A 5xx completion logs at Error and carries the request id; a 2xx logs at Info.
func TestRequestLogger_5xxIsError(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug}))
	r := gin.New()
	g := r.Group("/api/v1")
	g.Use(requestIDMiddleware())
	g.Use(requestLogger(logger))
	g.GET("/boom", func(c *gin.Context) { c.JSON(http.StatusInternalServerError, gin.H{"error": "x"}) })
	g.GET("/ok", func(c *gin.Context) { c.JSON(http.StatusOK, gin.H{"ok": true}) })

	r.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/api/v1/boom", nil))
	r.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/api/v1/ok", nil))

	logs := buf.String()
	require.Contains(t, logs, `"level":"ERROR"`, "a 5xx must log at Error")
	require.Contains(t, logs, `"level":"INFO"`, "a 2xx must log at Info")
	require.Contains(t, logs, `"request_id"`, "every line must carry the request id")
}
