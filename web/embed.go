// Package web provides the embedded frontend build output.
// The web/dist directory is populated by running "npm run build" inside
// the web/ directory (or "make web-build" / "make prod").
//
// If web/dist does not exist at build time, the embed produces an empty FS
// and static serving is disabled (use the Vite dev server instead).
package web

import "embed"

// DistFS embeds the production-built frontend assets.
// The "all:" prefix includes dot-files and directories that would normally be excluded.
//
//go:embed all:dist
var DistFS embed.FS
