package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"

	"libratrack/internal/auth"
)

const testSecret = "middleware-test-secret-at-least-32-chars"

// newRouter будує мінімальний роутер із тими самими групами, що й у застосунку.
func newRouter() *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	ok := func(c *gin.Context) { c.JSON(http.StatusOK, gin.H{"user_id": c.GetUint(CtxUserID)}) }

	protected := r.Group("/", AuthMiddleware())
	protected.GET("/me", ok)
	staff := protected.Group("", RequireRole("librarian", "admin"))
	staff.POST("/books", ok)
	admin := protected.Group("", RequireRole("admin"))
	admin.GET("/users", ok)
	return r
}

func expiredToken(t *testing.T) string {
	t.Helper()
	claims := auth.Claims{UserID: 1, Role: "reader", RegisteredClaims: jwt.RegisteredClaims{
		ExpiresAt: jwt.NewNumericDate(time.Now().Add(-time.Minute)),
	}}
	s, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(testSecret))
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func TestAuthAndRBAC(t *testing.T) {
	t.Setenv("JWT_SECRET", testSecret)
	router := newRouter()

	tokenFor := func(id uint, role string) string {
		tok, err := auth.GenerateToken(id, role)
		if err != nil {
			t.Fatal(err)
		}
		return tok
	}

	tests := []struct {
		name   string
		method string
		path   string
		header string
		want   int
	}{
		{"без заголовка -> 401", "GET", "/me", "", http.StatusUnauthorized},
		{"не Bearer -> 401", "GET", "/me", "Basic abc", http.StatusUnauthorized},
		{"сміття замість токена -> 401", "GET", "/me", "Bearer garbage", http.StatusUnauthorized},
		{"прострочений токен -> 401", "GET", "/me", "Bearer " + expiredToken(t), http.StatusUnauthorized},
		{"reader бачить /me", "GET", "/me", "Bearer " + tokenFor(1, "reader"), http.StatusOK},
		{"reader -> POST /books = 403", "POST", "/books", "Bearer " + tokenFor(1, "reader"), http.StatusForbidden},
		{"reader -> GET /users = 403", "GET", "/users", "Bearer " + tokenFor(1, "reader"), http.StatusForbidden},
		{"librarian -> POST /books = 200", "POST", "/books", "Bearer " + tokenFor(2, "librarian"), http.StatusOK},
		{"librarian -> GET /users = 403", "GET", "/users", "Bearer " + tokenFor(2, "librarian"), http.StatusForbidden},
		{"admin -> POST /books = 200", "POST", "/books", "Bearer " + tokenFor(3, "admin"), http.StatusOK},
		{"admin -> GET /users = 200", "GET", "/users", "Bearer " + tokenFor(3, "admin"), http.StatusOK},
		{"невідома роль -> 403", "POST", "/books", "Bearer " + tokenFor(4, "hacker"), http.StatusForbidden},
		{"без токена на staff -> 401, не 403", "POST", "/books", "", http.StatusUnauthorized},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, tt.path, nil)
			if tt.header != "" {
				req.Header.Set("Authorization", tt.header)
			}
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)
			if w.Code != tt.want {
				t.Errorf("status = %d, want %d; body=%s", w.Code, tt.want, w.Body)
			}
		})
	}
}

// RequireRole без AuthMiddleware (роль не встановлена) має віддавати 401, а не пропускати.
func TestRequireRole_WithoutAuthIsUnauthenticated(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	called := false
	r.GET("/x", RequireRole("admin"), func(c *gin.Context) { called = true })

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("GET", "/x", nil))
	if w.Code != http.StatusUnauthorized || called {
		t.Errorf("code=%d handlerCalled=%v, очікували 401 і що хендлер не викликано", w.Code, called)
	}
}
