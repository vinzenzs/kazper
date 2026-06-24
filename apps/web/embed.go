// Package webapp embeds the built coach-dashboard SPA so the Kazper binary can
// serve it at `/` with no external assets. The build artifact (dist) is
// committed (mirroring the docs/ swagger precedent), so `go build` needs no Node
// toolchain; `task web:build` regenerates it. See add-coach-dashboard.
package webapp

import (
	"embed"
	"io/fs"
)

// distFS holds the Vite build output. The `all:` prefix includes files whose
// names begin with `_` or `.` (Vite can emit such asset names).
//
//go:embed all:dist
var distFS embed.FS

// DistFS returns the embedded SPA build rooted at the dist directory (so
// "index.html" and "assets/..." resolve directly), and reports whether a real
// build is present. A checkout with only the placeholder index.html still
// returns ok=true; callers may serve it regardless.
func DistFS() (fs.FS, error) {
	return fs.Sub(distFS, "dist")
}
