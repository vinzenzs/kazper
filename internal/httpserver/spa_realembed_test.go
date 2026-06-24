//go:build webembed

package httpserver

import (
	"io/fs"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	webapp "github.com/vinzenzs/kazper/apps/web"
	"github.com/vinzenzs/kazper/internal/auth"
	"github.com/vinzenzs/kazper/internal/config"
)

// TestSPA_RealEmbeddedBuildServes drives RegisterSPA against the ACTUAL
// go:embed build (apps/web/dist), not a fixture fs — closing the gap between the
// fixture serving tests and what the compiled binary ships. It only compiles
// under `-tags webembed` (where the real dist is embedded), so the default
// `go test ./...` (which embeds the stub) skips it. CI runs it after
// `npm run build` via `go test -tags webembed ./internal/httpserver/...`.
func TestSPA_RealEmbeddedBuildServes(t *testing.T) {
	dist, err := webapp.DistFS()
	require.NoError(t, err)

	r := BuildEngine()
	r.Group(config.APIBasePath).GET("/ping", func(c *gin.Context) { c.Status(http.StatusOK) })
	require.NoError(t, RegisterSPA(r, dist, auth.Config{}))

	// Root serves the real shell.
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	require.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Header().Get("Content-Type"), "text/html")
	assert.Contains(t, rec.Body.String(), "<div id=\"root\">")

	// A real content-hashed asset under /assets serves from the embedded build.
	var asset string
	require.NoError(t, fs.WalkDir(dist, "assets", func(p string, d fs.DirEntry, walkErr error) error {
		if walkErr == nil && !d.IsDir() && asset == "" {
			asset = p
		}
		return nil
	}))
	require.NotEmpty(t, asset, "expected at least one built asset under dist/assets")

	rec = httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/"+asset, nil))
	assert.Equal(t, http.StatusOK, rec.Code)
}
