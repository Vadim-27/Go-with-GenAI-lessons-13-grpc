package middleware

import (
	"net/http"
	"slices"

	"github.com/gin-gonic/gin"

	"libratrack/internal/auth"
)

// AuthCookieName — cookie, у якій веб-інтерфейс зберігає JWT.
//
// Cookie має атрибут HttpOnly: JavaScript сторінки (у тому числі впроваджений
// через XSS) не може її прочитати, на відміну від localStorage.
const AuthCookieName = "libratrack_token"

// SetAuthCookie записує JWT у cookie: HttpOnly, SameSite=Strict, Path=/,
// термін дії дорівнює терміну дії токена. secure додає атрибут Secure
// (cookie надсилається лише через HTTPS; localhost браузери вважають безпечним).
func SetAuthCookie(c *gin.Context, token string, secure bool) {
	http.SetCookie(c.Writer, &http.Cookie{
		Name:     AuthCookieName,
		Value:    token,
		Path:     "/",
		MaxAge:   int(auth.TokenTTL.Seconds()),
		HttpOnly: true,
		Secure:   secure,
		SameSite: http.SameSiteStrictMode,
	})
}

// ClearAuthCookie видаляє cookie з токеном (вихід із системи).
func ClearAuthCookie(c *gin.Context, secure bool) {
	http.SetCookie(c.Writer, &http.Cookie{
		Name:     AuthCookieName,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   secure,
		SameSite: http.SameSiteStrictMode,
	})
}

// WebAuth — "м'яка" автентифікація для HTML-сторінок.
//
// Якщо cookie містить дійсний токен, кладе user_id і role в контекст (ті самі
// ключі, що й AuthMiddleware). Якщо cookie немає або токен недійсний — запит іде
// далі як анонімний: каталог доступний усім, а захищені маршрути закриває
// RequireWebRole.
func WebAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		if raw, err := c.Cookie(AuthCookieName); err == nil && raw != "" {
			if claims, err := auth.ParseToken(raw); err == nil {
				c.Set(CtxUserID, claims.UserID)
				c.Set(CtxRole, claims.Role)
			}
		}
		c.Next()
	}
}

// RequireWebRole — RBAC для веб-маршрутів (аналог RequireRole, але без JSON).
//
//   - не автентифікований: звичайний GET переадресовується на /login (303),
//     інші запити (HTMX, POST, DELETE) отримують 401 і заголовок HX-Redirect;
//   - роль не дозволена -> 403 Forbidden.
//
// В обох випадках ланцюжок переривається, хендлер не виконується.
func RequireWebRole(allowed ...string) gin.HandlerFunc {
	return func(c *gin.Context) {
		value, exists := c.Get(CtxRole)
		if !exists {
			if c.Request.Method == http.MethodGet && !IsHTMX(c) {
				c.Redirect(http.StatusSeeOther, "/login")
				c.Abort()
				return
			}
			c.Header("HX-Redirect", "/login")
			c.String(http.StatusUnauthorized, "Потрібно увійти в систему")
			c.Abort()
			return
		}

		role, _ := value.(string)
		if !slices.Contains(allowed, role) {
			c.String(http.StatusForbidden, "Недостатньо прав: потрібна роль librarian або admin")
			c.Abort()
			return
		}
		c.Next()
	}
}

// IsHTMX повідомляє, чи запит надіслала бібліотека HTMX (заголовок HX-Request).
func IsHTMX(c *gin.Context) bool {
	return c.GetHeader("HX-Request") == "true"
}
