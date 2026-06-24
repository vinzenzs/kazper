//go:build webembed

package webapp

import (
	"embed"
	"io/fs"
)

// distFS holds the real Vite build output (apps/web/dist), embedded only under
// `-tags webembed`. dist/ is gitignored and must be present at compile time —
// produced by `task web:build` or the Docker node stage. The `all:` prefix
// includes files whose names begin with `_` or `.` (Vite can emit such names).
//
//go:embed all:dist
var distFS embed.FS

// DistFS returns the embedded SPA build rooted at dist (so "index.html" and
// "assets/..." resolve directly).
func DistFS() (fs.FS, error) {
	return fs.Sub(distFS, "dist")
}
