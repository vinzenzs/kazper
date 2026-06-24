//go:build !webembed

// Package webapp embeds the coach-dashboard SPA so the Kazper binary serves it
// at `/` with no external assets.
//
// Two build modes (per build-spa-in-pipeline):
//   - default (this file): embeds the committed placeholder shell in stub/, so
//     `go build` / `go test` / `go vet` compile with no Node toolchain and
//     nothing generated is committed.
//   - `-tags webembed` (embed_dist.go): embeds the real Vite build in dist/,
//     which is gitignored and produced by `task web:build` or the Docker node
//     stage. The release image always builds with this tag.
package webapp

import (
	"embed"
	"io/fs"
)

//go:embed all:stub
var distFS embed.FS

// DistFS returns the embedded SPA rooted so "index.html" and "assets/..."
// resolve directly. In the default build this is the placeholder shell.
func DistFS() (fs.FS, error) {
	return fs.Sub(distFS, "stub")
}
