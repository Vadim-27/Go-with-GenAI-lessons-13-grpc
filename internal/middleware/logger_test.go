package middleware

import (
	"bytes"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

// Logger не повинен писати в лог тіло запиту, query-рядок і заголовок Authorization:
// там можуть бути пароль, токен тощо.
func TestLogger_DoesNotLogSecrets(t *testing.T) {
	gin.SetMode(gin.TestMode)

	var buf bytes.Buffer
	log.SetOutput(&buf)
	t.Cleanup(func() { log.SetOutput(os.Stderr) })

	r := gin.New()
	r.Use(Logger())
	r.POST("/auth/login", func(c *gin.Context) { c.Status(http.StatusOK) })

	body := strings.NewReader(`{"email":"a@b.c","password":"SuperSecretPassword123"}`)
	req := httptest.NewRequest(http.MethodPost, "/auth/login?password=QuerySecret&token=QueryToken", body)
	req.Header.Set("Authorization", "Bearer header.payload.signature")
	r.ServeHTTP(httptest.NewRecorder(), req)

	out := buf.String()
	if !strings.Contains(out, "/auth/login") {
		t.Fatalf("запит має логуватись, лог: %q", out)
	}
	for _, secret := range []string{"SuperSecretPassword123", "QuerySecret", "QueryToken", "header.payload.signature"} {
		if strings.Contains(out, secret) {
			t.Errorf("лог містить секрет %q: %s", secret, out)
		}
	}
}
