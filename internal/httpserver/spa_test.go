package httpserver

import (
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"testing"
	"testing/fstest"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/vinzenzs/kazper/internal/auth"
	"github.com/vinzenzs/kazper/internal/config"
)

// testDist is a stand-in for the embedded build so these serving tests don't
// depend on a real `vite build`. RegisterSPA is given exactly this fs.
func testDist() fstest.MapFS {
	return fstest.MapFS{
		"index.html":         {Data: []byte("<!doctype html><title>shell</title>")},
		"assets/app-abc.js":  {Data: []byte("console.log('app')")},
		"assets/app-abc.css": {Data: []byte(".x{}")},
	}
}

// spaEngine mirrors Run()'s wiring: infra route, an /api/v1 group route, then
// RegisterSPA registered last so it only owns NoRoute fallthroughs.
func spaEngine(t *testing.T, authCfg auth.Config) *gin.Engine {
	t.Helper()
	r := BuildEngine()
	r.GET("/healthz", func(c *gin.Context) { c.JSON(http.StatusOK, gin.H{"status": "ok"}) })
	r.Group(config.APIBasePath).GET("/ping", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"pong": true})
	})
	require.NoError(t, RegisterSPA(r, testDist(), authCfg))
	return r
}

func TestSPA_ShellServedAtRoot(t *testing.T) {
	r := spaEngine(t, auth.Config{})
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))

	require.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Header().Get("Content-Type"), "text/html")
	assert.Contains(t, rec.Body.String(), "shell")
}

func TestSPA_StaticAssetServed(t *testing.T) {
	r := spaEngine(t, auth.Config{})
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/assets/app-abc.js", nil))

	require.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), "console.log")
}

func TestSPA_UnknownRouteFallsBackToShell(t *testing.T) {
	r := spaEngine(t, auth.Config{})
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/some/client/route", nil))

	require.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), "shell")
}

func TestSPA_UnknownAPIPathReturnsJSON404(t *testing.T) {
	r := spaEngine(t, auth.Config{})
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, config.APIBasePath+"/nope", nil))

	require.Equal(t, http.StatusNotFound, rec.Code)
	assert.Equal(t, "application/json; charset=utf-8", rec.Header().Get("Content-Type"))
	assert.JSONEq(t, `{"error":"not_found"}`, rec.Body.String())
}

func TestSPA_InfraEndpointUntouched(t *testing.T) {
	r := spaEngine(t, auth.Config{})
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/healthz", nil))
	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestSPA_WebGatePromptsWhenEnabled(t *testing.T) {
	cfg := auth.Config{WebUser: "coach", WebPassword: "dashboard-pass-dddddddddddd"}
	r := spaEngine(t, cfg)

	// No credential → 401 + WWW-Authenticate so the browser prompts.
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	require.Equal(t, http.StatusUnauthorized, rec.Code)
	assert.Equal(t, auth.BasicRealm, rec.Header().Get("WWW-Authenticate"))

	// Valid credential → shell served.
	rec = httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Basic "+base64.StdEncoding.EncodeToString([]byte("coach:dashboard-pass-dddddddddddd")))
	r.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), "shell")
}

func TestSPA_WebGateStillReturnsAPI404Unauthenticated(t *testing.T) {
	// The API-404 contract precedes the web gate: an unknown /api/v1 path is a
	// JSON 404 even when web auth is enabled and no credential is presented.
	cfg := auth.Config{WebUser: "coach", WebPassword: "dashboard-pass-dddddddddddd"}
	r := spaEngine(t, cfg)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, config.APIBasePath+"/nope", nil))
	require.Equal(t, http.StatusNotFound, rec.Code)
	assert.JSONEq(t, `{"error":"not_found"}`, rec.Body.String())
}
