package testutil

import (
	"net/http"
	"testing"

	"gorm.io/gorm"

	"libratrack/internal/app"
	"libratrack/web"
)

// NewRouter збирає справжній роутер застосунку з ВБУДОВАНИМИ (go:embed)
// шаблонами та статикою — так само, як cmd/api/main.go.
func NewRouter(t *testing.T, db *gorm.DB) http.Handler {
	t.Helper()

	static, err := web.Static()
	if err != nil {
		t.Fatal(err)
	}
	router, err := app.NewRouter(db, app.Web{Templates: web.TemplatesFS, Static: static})
	if err != nil {
		t.Fatal(err)
	}
	return router
}
