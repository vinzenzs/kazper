package httpserver

import (
	"io/fs"
	"net/http"
	"path"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/vinzenzs/kazper/internal/auth"
	"github.com/vinzenzs/kazper/internal/config"
)

// RegisterSPA installs same-origin serving of the embedded coach-dashboard SPA
// on r. distFS must be rooted at the build output (so "index.html" and
// "assets/..." resolve directly). Resolution order for an incoming request:
//
//   - registered routes (the /api/v1 group + /healthz,/readyz,/swagger infra)
//     always win — RegisterSPA only handles what falls through to NoRoute;
//   - an unknown path under /api/v1 keeps the JSON not_found contract (the SPA
//     fallback never shadows API 404s);
//   - a GET matching a real file in dist is served from the embedded build;
//   - any other non-API GET returns index.html for client-side routing.
//
// When the web identity is configured (WEB_USER/WEB_PASSWORD), the shell and its
// assets are gated by the same Basic realm as the API: a request without a valid
// credential gets 401 + WWW-Authenticate so the browser prompts once (a top-level
// navigation to "/" triggers the native dialog; the cached credential is then
// auto-attached to the SPA's same-origin API fetches). When the web identity is
// not configured the shell is served openly (its API calls will simply 401).
func RegisterSPA(r *gin.Engine, distFS fs.FS, authCfg auth.Config) error {
	index, err := fs.ReadFile(distFS, "index.html")
	if err != nil {
		return err
	}
	fileServer := http.FileServer(http.FS(distFS))
	r.NoRoute(spaFallback(distFS, fileServer, index, authCfg))
	return nil
}

func spaFallback(distFS fs.FS, fileServer http.Handler, index []byte, authCfg auth.Config) gin.HandlerFunc {
	webEnabled := auth.WebEnabled(authCfg)
	return func(c *gin.Context) {
		p := c.Request.URL.Path

		// API 404s and any non-GET unknown route keep the JSON not_found contract.
		// This check precedes the web gate so an unknown /api/v1 path is a 404, not
		// a Basic-auth challenge.
		if c.Request.Method != http.MethodGet ||
			p == config.APIBasePath || strings.HasPrefix(p, config.APIBasePath+"/") {
			c.JSON(http.StatusNotFound, gin.H{"error": "not_found"})
			return
		}

		// Gate the dashboard shell + assets behind the shared Basic realm.
		if webEnabled && !auth.ValidWebBasic(c.GetHeader("Authorization"), authCfg) {
			c.Header("WWW-Authenticate", auth.BasicRealm)
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "auth_required"})
			return
		}

		// Serve a real static file when the path maps to one in dist.
		if name := strings.TrimPrefix(path.Clean(p), "/"); name != "" {
			if f, openErr := distFS.Open(name); openErr == nil {
				info, statErr := f.Stat()
				_ = f.Close()
				if statErr == nil && !info.IsDir() {
					fileServer.ServeHTTP(c.Writer, c.Request)
					return
				}
			}
		}

		// Otherwise it's the root or a client-side route → serve the SPA shell.
		c.Data(http.StatusOK, "text/html; charset=utf-8", index)
	}
}
