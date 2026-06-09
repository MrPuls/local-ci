// Package web embeds the built single-page app so `local-ci ui` can serve the
// whole UI from one binary.
//
// The Vite build (web/, `bun run build`) outputs into internal/web/dist, which
// is committed and embedded below. Rebuild and re-commit dist whenever the
// frontend changes; `go build` needs no extra flags.
package web

import (
	"embed"
	"io/fs"
)

//go:embed all:dist
var dist embed.FS

// Dist returns the built SPA rooted at its top level (index.html at the root).
func Dist() (fs.FS, error) { return fs.Sub(dist, "dist") }
