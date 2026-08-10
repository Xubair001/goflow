// Package web embeds the built React dashboard (frontend/) so the
// apiserver binary can serve it alongside its API — one binary, no
// separate static file host in production.
//
// dist/ starts out as the placeholder page checked into git; run
// `make frontend-build` (or `cd frontend && npm run build`, which is
// configured to output straight into this dist/ directory) to replace it
// with the real built app before compiling apiserver for production.
package web

import (
	"embed"
	"io/fs"
)

//go:embed all:dist
var distFS embed.FS

// DistFS returns the embedded frontend build, rooted at dist/ rather than
// dist/'s parent, so callers can serve it directly as a file system.
func DistFS() (fs.FS, error) {
	return fs.Sub(distFS, "dist")
}
