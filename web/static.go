package web

import (
	"embed"
	"io/fs"
)

//go:embed static
var staticFS embed.FS

// Static повертає вбудовані статичні файли без префікса "static/":
// файл static/css/app.css доступний як css/app.css і роздається за /static/css/app.css.
func Static() (fs.FS, error) {
	return fs.Sub(staticFS, "static")
}
