package httpserver

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

// With metrics not wired, /metrics falls through to the JSON 404 (the default,
// which is what an install without METRICS_ENABLED serves).
func TestMetrics_DisabledIs404(t *testing.T) {
	r := BuildEngine()
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/metrics", nil))
	require.Equal(t, http.StatusNotFound, rec.Code)
}

// Enabled, /metrics serves request series labeled by the route TEMPLATE — never
// the raw id-bearing path.
func TestMetrics_EnabledExposesRouteTemplateSeries(t *testing.T) {
	r := BuildEngine()
	m := newMetrics()
	r.Use(m.middleware())
	r.GET("/metrics", gin.WrapH(m.handler()))
	api := r.Group("/api/v1")
	api.GET("/meals/:id", func(c *gin.Context) { c.JSON(http.StatusOK, gin.H{"ok": true}) })

	// A request with a concrete id.
	r.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/api/v1/meals/abc-123", nil))

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/metrics", nil))
	require.Equal(t, http.StatusOK, rec.Code)

	body := rec.Body.String()
	require.Contains(t, body, "http_request_duration_seconds")
	require.Contains(t, body, "http_requests_total")
	require.Contains(t, body, `route="/api/v1/meals/:id"`, "route label must be the template")
	require.Contains(t, body, `status="2xx"`, "status is collapsed to its class")
	require.NotContains(t, body, "abc-123", "the raw id must never appear as a label value")
	// Runtime collectors ship for free.
	require.Contains(t, body, "go_goroutines")
}
