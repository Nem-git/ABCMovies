package serving

import (
	"embed"
	"io/fs"
	"net/http"
)

// distFS holds the built page (index.html + bundle.js), produced by
// `make web-build` from ../src/. The build artifacts are not committed; the
// Makefile chains web-build before every Go compile of this package.
//
//go:embed all:dist
var distFS embed.FS

func staticHandler() http.Handler {
	sub, err := fs.Sub(distFS, "dist")
	if err != nil {
		panic("web: embedded dist missing: " + err.Error())
	}
	return http.FileServerFS(sub)
}
