package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// contentSecurityPolicy дозволяє скрипти, стилі й шрифти ЛИШЕ з нашого
// origin (вбудована статика). Inline-скрипти та eval заборонені, тому навіть
// якщо XSS обійде автоекранування, браузер не виконає впроваджений <script>.
const contentSecurityPolicy = "default-src 'self'; script-src 'self'; style-src 'self'; " +
	"img-src 'self' data:; object-src 'none'; base-uri 'self'; form-action 'self'; frame-ancestors 'none'"

// SecurityHeaders додає заголовки безпеки до HTML-відповідей веб-інтерфейсу.
// Cache-Control: no-store — сторінки залежать від ролі користувача і не мають
// осідати в спільних кешах.
func SecurityHeaders() gin.HandlerFunc {
	return func(c *gin.Context) {
		h := c.Writer.Header()
		h.Set("Content-Security-Policy", contentSecurityPolicy)
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("X-Frame-Options", "DENY")
		h.Set("Referrer-Policy", "same-origin")
		h.Set("Cache-Control", "no-store")
		c.Next()
	}
}

// SameOrigin — захист від CSRF для cookie-автентифікації.
//
// Браузер автоматично додає cookie до запитів, тому сторонній сайт міг би
// відправити форму POST /web/books/1 від імені користувача. Перша лінія — cookie
// SameSite=Strict; друга — ця перевірка: небезпечні методи (POST, PUT, DELETE)
// з іншого origin (заголовки Sec-Fetch-Site / Origin) відхиляються з 403.
// GET/HEAD/OPTIONS не перевіряються — вони не змінюють стан.
func SameOrigin() gin.HandlerFunc {
	protection := http.NewCrossOriginProtection()
	return func(c *gin.Context) {
		if err := protection.Check(c.Request); err != nil {
			c.String(http.StatusForbidden, "Запит з іншого сайту відхилено")
			c.Abort()
			return
		}
		c.Next()
	}
}
