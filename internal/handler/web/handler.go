package web

import (
	"io/fs"
	"log"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"libratrack/internal/middleware"
	"libratrack/internal/model"
	"libratrack/internal/repository"
)

// maxFormBytes — максимальний розмір тіла HTML-форми (64 КБ).
const maxFormBytes = 64 << 10

// Repos — сховища, з якими працює веб-інтерфейс.
type Repos struct {
	Books      repository.BookRepository
	Copies     repository.CopyRepository
	Categories repository.CategoryRepository
	Authors    repository.AuthorRepository
	Users      repository.UserRepository
}

// Handler обслуговує HTML-сторінки та HTMX-фрагменти.
type Handler struct {
	repos Repos
	view  *Renderer
	// secureCookie додає атрибут Secure до cookie з JWT.
	secureCookie bool
}

// New повертає веб-хендлер.
func New(repos Repos, view *Renderer, secureCookie bool) *Handler {
	return &Handler{repos: repos, view: view, secureCookie: secureCookie}
}

// Register підключає веб-маршрути.
//
// Ланцюжок: SecurityHeaders -> SameOrigin (CSRF) -> WebAuth (cookie) ->
// [RequireWebRole для операцій запису]. WebAuth стоїть перед RequireWebRole,
// бо RBAC читає роль, яку кладе саме він.
func (h *Handler) Register(r gin.IRouter) {
	g := r.Group("", middleware.SecurityHeaders(), middleware.SameOrigin(), middleware.WebAuth())

	g.GET("/", func(c *gin.Context) { c.Redirect(http.StatusSeeOther, "/catalog") })
	g.GET("/catalog", h.Catalog)
	g.GET("/web/books", h.BookList) // HTMX live search: лише рядки таблиці

	g.GET("/login", h.LoginPage)
	g.POST("/login", h.Login)
	g.GET("/register", h.SignUpPage)
	g.POST("/register", h.SignUp)
	g.POST("/logout", h.Logout)

	staff := g.Group("", middleware.RequireWebRole(model.RoleLibrarian, model.RoleAdmin))
	staff.GET("/web/books/new", h.NewBookForm)
	staff.POST("/web/books", h.CreateBook)
	staff.GET("/web/books/:id/edit", h.EditBookForm)
	staff.POST("/web/books/:id", h.UpdateBook)
	staff.DELETE("/web/books/:id", h.DeleteBook) // HTMX: 200 з порожнім тілом
}

// StaticHandler роздає вбудовані статичні файли за /static/*filepath.
// Перелік вмісту каталогів вимкнено: шлях із "/" в кінці дає 404.
func StaticHandler(static fs.FS) gin.HandlerFunc {
	files := http.StripPrefix("/static", http.FileServerFS(static))
	return func(c *gin.Context) {
		if strings.HasSuffix(c.Request.URL.Path, "/") {
			c.String(http.StatusNotFound, "Not found")
			return
		}
		c.Header("Cache-Control", "public, max-age=3600")
		c.Header("X-Content-Type-Options", "nosniff")
		files.ServeHTTP(c.Writer, c.Request)
	}
}

// pageData — спільні поля всіх сторінок (шапка, flash-повідомлення).
type pageData struct {
	Title     string
	LoggedIn  bool
	Role      string
	CanManage bool // librarian або admin: показувати кнопки керування
	Flash     string
	Content   any
}

// page заповнює pageData з контексту запиту (роль кладе WebAuth).
func (h *Handler) page(c *gin.Context, title string, content any) pageData {
	role := c.GetString(middleware.CtxRole)
	return pageData{
		Title:     title,
		LoggedIn:  role != "",
		Role:      role,
		CanManage: canManage(role),
		Flash:     flashMessages[c.Query("msg")],
		Content:   content,
	}
}

// flashMessages — фіксований набір повідомлень після переадресації. У URL лише
// ключ (?msg=created), тому через параметр не можна підсунути довільний текст.
var flashMessages = map[string]string{
	"created":    "Книгу додано",
	"updated":    "Зміни збережено",
	"logged_in":  "Ви увійшли в систему",
	"logged_out": "Ви вийшли з системи",
	"registered": "Акаунт створено, ви увійшли як reader",
}

func canManage(role string) bool {
	return role == model.RoleLibrarian || role == model.RoleAdmin
}

// errorPage показує сторінку помилки (для HTMX — короткий текст).
func (h *Handler) errorPage(c *gin.Context, status int, message string) {
	if middleware.IsHTMX(c) {
		c.String(status, message)
		return
	}
	h.view.Page(c, status, "error.html", h.page(c, http.StatusText(status), errorData{Status: status, Message: message}))
}

type errorData struct {
	Status  int
	Message string
}

// serverError логує деталі й показує загальне повідомлення без внутрішніх подробиць.
func (h *Handler) serverError(c *gin.Context, err error) {
	log.Printf("web: %s %s: %v", c.Request.Method, c.Request.URL.Path, err)
	h.errorPage(c, http.StatusInternalServerError, "Внутрішня помилка сервера")
}

// limitForm обмежує розмір тіла форми і розбирає її. false — відповідь уже надіслано.
func (h *Handler) limitForm(c *gin.Context) bool {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxFormBytes)
	if err := c.Request.ParseForm(); err != nil {
		h.errorPage(c, http.StatusBadRequest, "Некоректні дані форми")
		return false
	}
	return true
}
