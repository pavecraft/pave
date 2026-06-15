// Package uistatic holds the compiled React/Vite UI bundle embedded at build
// time. Run "cd ui && npm run build" before "go build" to populate dist/.
package uistatic

import "embed"

//go:embed all:dist
var Files embed.FS
