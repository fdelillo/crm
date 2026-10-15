// Package web serves the embedded frontend independently of the API router.
package web

import (
	"embed"
	"io/fs"
)

//go:embed all:dist
var distFiles embed.FS

// DistFS returns the built frontend, or just its marker in a backend-only checkout.
func DistFS() fs.FS {
	dist, err := fs.Sub(distFiles, "dist")
	if err != nil {
		panic(err) // The embedded dist directory is checked at compile time.
	}
	return dist
}
