// Інтеграційні тести веб-інтерфейсу: справжній роутер (із вбудованими через
// go:embed шаблонами й статикою) + справжня PostgreSQL (TEST_DATABASE_URL),
// запити через httptest. Без TEST_DATABASE_URL тести пропускаються.
package web_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"gorm.io/gorm"

	"libratrack/internal/auth"
	"libratrack/internal/middleware"
	"libratrack/internal/model"
	"libratrack/internal/repository"
	"libratrack/internal/testutil"
)

// Окрема схема БД: пакети тестів Go виконуються паралельно.
const schemaName = "t_web"

const testPassword = "CorrectPassword123!"

// env — роутер, БД, демо-каталог і по користувачу кожної ролі.
type env struct {
	t      *testing.T
	db     *gorm.DB
	router http.Handler
	users  map[string]model.User // за роллю
	books  map[string]uint       // назва -> id
	cat    uint
	author uint
}

func newEnv(t *testing.T) *env {
	t.Helper()
	t.Setenv("JWT_SECRET", "web-test-secret-at-least-32-characters!!")

	db := testutil.NewDB(t, schemaName)
	e := &env{t: t, db: db, router: testutil.NewRouter(t, db), users: map[string]model.User{}, books: map[string]uint{}}

	hash, err := auth.HashPassword(testPassword)
	if err != nil {
		t.Fatal(err)
	}
	for _, role := range []string{model.RoleReader, model.RoleLibrarian, model.RoleAdmin} {
		u := model.User{Email: role + "@libratrack.com", PasswordHash: hash, Role: role}
		if err := db.Create(&u).Error; err != nil {
			t.Fatal(err)
		}
		e.users[role] = u
	}

	programming := model.Category{Name: "Programming"}
	scifi := model.Category{Name: "Science Fiction"}
	donovan := model.Author{FullName: "Alan Donovan"}
	herbert := model.Author{FullName: "Frank Herbert"}
	for _, row := range []any{&programming, &scifi, &donovan, &herbert} {
		if err := db.Create(row).Error; err != nil {
			t.Fatal(err)
		}
	}
	e.cat, e.author = programming.ID, donovan.ID

	e.addBook("The Go Programming Language", "9780134190440", programming.ID, donovan.ID, "available", "borrowed")
	e.addBook("Dune", "9780441013593", scifi.ID, herbert.ID, "borrowed")
	e.addBook("Clean Code", "9780132350884", programming.ID, donovan.ID)
	return e
}

// addBook створює книгу з одним автором і примірниками з указаними статусами.
func (e *env) addBook(title, isbn string, categoryID, authorID uint, statuses ...string) uint {
	e.t.Helper()
	book := model.Book{Title: title, ISBN: isbn, CategoryID: categoryID}
	if err := repository.NewBookRepository(e.db).Create(context.Background(), &book, []uint{authorID}); err != nil {
		e.t.Fatal(err)
	}
	for i, status := range statuses {
		cp := model.Copy{BookID: book.ID, InventoryNo: fmt.Sprintf("%s-%d", isbn, i), Status: status}
		if err := e.db.Create(&cp).Error; err != nil {
			e.t.Fatal(err)
		}
	}
	e.books[title] = book.ID
	return book.ID
}

// request — опис одного HTTP-запиту в таблиці.
type request struct {
	method string
	path   string
	role   string            // "" — анонім; інакше cookie з JWT цієї ролі
	cookie string            // довільне значення cookie (перекриває role)
	header map[string]string // додаткові заголовки (HX-Request, Origin...)
}

func (e *env) do(r request) *httptest.ResponseRecorder {
	e.t.Helper()
	req := httptest.NewRequest(r.method, r.path, nil)
	for k, v := range r.header {
		req.Header.Set(k, v)
	}
	switch {
	case r.cookie != "":
		req.AddCookie(&http.Cookie{Name: middleware.AuthCookieName, Value: r.cookie})
	case r.role != "":
		u := e.users[r.role]
		token, err := auth.GenerateToken(u.ID, u.Role)
		if err != nil {
			e.t.Fatal(err)
		}
		req.AddCookie(&http.Cookie{Name: middleware.AuthCookieName, Value: token})
	}
	rec := httptest.NewRecorder()
	e.router.ServeHTTP(rec, req)
	return rec
}

var htmx = map[string]string{"HX-Request": "true"}

// checkBody перевіряє наявність і відсутність підрядків у тілі відповіді.
func checkBody(t *testing.T, body string, contains, absent []string) {
	t.Helper()
	for _, s := range contains {
		if !strings.Contains(body, s) {
			t.Errorf("тіло не містить %q", s)
		}
	}
	for _, s := range absent {
		if strings.Contains(body, s) {
			t.Errorf("тіло НЕ повинно містити %q", s)
		}
	}
}

// Тест 1: GET /catalog -> 200, повна сторінка: layout + таблиця книг.
func TestCatalogPage(t *testing.T) {
	e := newEnv(t)

	tests := []struct {
		name     string
		req      request
		status   int
		contains []string
		absent   []string
	}{
		{
			name:   "анонім бачить layout, пошук і таблицю книг",
			req:    request{method: http.MethodGet, path: "/catalog"},
			status: http.StatusOK,
			contains: []string{
				"<!doctype html>", "<header class=\"topbar\">", "<footer", // layout.html
				`<link rel="stylesheet" href="/static/css/app.css">`,
				`<script src="/static/js/htmx.min.js" defer></script>`,
				`<table class="books">`, `<tbody id="book-list">`,
				`hx-get="/web/books"`, `hx-trigger="keyup changed delay:300ms"`, `hx-target="#book-list"`,
				"The Go Programming Language", "Dune", "Clean Code",
				"доступно 1 з 2",    // індикатор наявності: Go
				"усі видані (1)",    // Dune
				"немає примірників", // Clean Code
				`href="/login"`,
			},
			absent: []string{"hx-delete", "/web/books/new"}, // кнопки керування лише для staff
		},
		{
			name:     "reader не бачить кнопок керування",
			req:      request{method: http.MethodGet, path: "/catalog", role: model.RoleReader},
			status:   http.StatusOK,
			contains: []string{`<table class="books">`, `<span class="role role-reader">reader</span>`},
			absent:   []string{"hx-delete", "/web/books/new"},
		},
		{
			name:   "librarian бачить кнопки видалення з атрибутами HTMX",
			req:    request{method: http.MethodGet, path: "/catalog", role: model.RoleLibrarian},
			status: http.StatusOK,
			contains: []string{
				fmt.Sprintf(`hx-delete="/web/books/%d"`, e.books["Dune"]),
				`hx-target="closest tr"`, `hx-swap="outerHTML"`, `hx-confirm="Delete this book?"`,
				"/web/books/new",
			},
		},
		{
			name:     "пошук без JavaScript: /catalog?q= фільтрує на сервері",
			req:      request{method: http.MethodGet, path: "/catalog?q=dune"},
			status:   http.StatusOK,
			contains: []string{"<!doctype html>", "Dune", `value="dune"`},
			absent:   []string{"Clean Code"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			rec := e.do(tc.req)
			if rec.Code != tc.status {
				t.Fatalf("статус = %d, очікувався %d: %s", rec.Code, tc.status, rec.Body)
			}
			if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "text/html") {
				t.Errorf("Content-Type = %q, очікувався text/html", ct)
			}
			if csp := rec.Header().Get("Content-Security-Policy"); !strings.Contains(csp, "script-src 'self'") {
				t.Errorf("немає CSP: %q", csp)
			}
			checkBody(t, rec.Body.String(), tc.contains, tc.absent)
		})
	}
}

// Тест 2: HTMX-запит GET /web/books?q=Go повертає ЛИШЕ фрагменти book_row.
func TestBookListFragment(t *testing.T) {
	e := newEnv(t)
	layout := []string{"<!doctype html>", "<html", "<head", "<header", "<table", "<footer"}

	tests := []struct {
		name     string
		req      request
		status   int
		contains []string
		absent   []string
	}{
		{
			name:     "q=Go: лише рядки <tr> з книгою про Go",
			req:      request{method: http.MethodGet, path: "/web/books?q=Go", header: htmx},
			status:   http.StatusOK,
			contains: []string{fmt.Sprintf(`<tr id="book-%d">`, e.books["The Go Programming Language"]), "The Go Programming Language"},
			absent:   append([]string{"Dune", "Clean Code"}, layout...),
		},
		{
			name:     "пошук за автором (Herbert) знаходить Dune",
			req:      request{method: http.MethodGet, path: "/web/books?q=herbert", header: htmx},
			status:   http.StatusOK,
			contains: []string{"Dune"},
			absent:   append([]string{"The Go Programming Language"}, layout...),
		},
		{
			name:     "пошук за ISBN",
			req:      request{method: http.MethodGet, path: "/web/books?q=9780132350884", header: htmx},
			status:   http.StatusOK,
			contains: []string{"Clean Code"},
			absent:   []string{"Dune"},
		},
		{
			name:     "порожній q повертає всі книги",
			req:      request{method: http.MethodGet, path: "/web/books?q=", header: htmx},
			status:   http.StatusOK,
			contains: []string{"The Go Programming Language", "Dune", "Clean Code"},
			absent:   layout,
		},
		{
			name:     "символ % не є шаблоном LIKE",
			req:      request{method: http.MethodGet, path: "/web/books?q=" + url.QueryEscape("%"), header: htmx},
			status:   http.StatusOK,
			contains: []string{`<tr class="empty">`, "Нічого не знайдено"},
			absent:   []string{"Dune"},
		},
		{
			name:     "фрагмент для librarian містить кнопку hx-delete",
			req:      request{method: http.MethodGet, path: "/web/books?q=Go", header: htmx, role: model.RoleLibrarian},
			status:   http.StatusOK,
			contains: []string{fmt.Sprintf(`hx-delete="/web/books/%d"`, e.books["The Go Programming Language"])},
		},
		{
			name:   "без HX-Request — переадресація на повну сторінку",
			req:    request{method: http.MethodGet, path: "/web/books?q=Go"},
			status: http.StatusSeeOther,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			rec := e.do(tc.req)
			if rec.Code != tc.status {
				t.Fatalf("статус = %d, очікувався %d: %s", rec.Code, tc.status, rec.Body)
			}
			checkBody(t, rec.Body.String(), tc.contains, tc.absent)
		})
	}

	t.Run("без HX-Request Location веде на /catalog?q=Go", func(t *testing.T) {
		rec := e.do(request{method: http.MethodGet, path: "/web/books?q=Go"})
		if loc := rec.Header().Get("Location"); loc != "/catalog?q=Go" {
			t.Errorf("Location = %q", loc)
		}
	})
}

// Тест 3: RBAC на hx-delete. Анонім -> 401, reader -> 403 (книга лишається),
// librarian/admin -> 200 з порожнім тілом (рядок зникає), CSRF -> 403.
func TestDeleteBook_RBAC(t *testing.T) {
	e := newEnv(t)

	tests := []struct {
		name    string
		req     request
		status  int
		deleted bool
	}{
		{name: "анонім (без cookie) -> 401", req: request{header: htmx}, status: http.StatusUnauthorized},
		{name: "підроблений токен у cookie -> 401", req: request{header: htmx, cookie: "forged.jwt.token"}, status: http.StatusUnauthorized},
		{name: "reader -> 403 Forbidden", req: request{header: htmx, role: model.RoleReader}, status: http.StatusForbidden},
		{
			name:   "librarian з чужого сайту (CSRF) -> 403",
			req:    request{role: model.RoleLibrarian, header: map[string]string{"HX-Request": "true", "Origin": "https://evil.example", "Sec-Fetch-Site": "cross-site"}},
			status: http.StatusForbidden,
		},
		{name: "librarian -> 200, порожнє тіло", req: request{header: htmx, role: model.RoleLibrarian}, status: http.StatusOK, deleted: true},
		{name: "admin -> 200, порожнє тіло", req: request{header: htmx, role: model.RoleAdmin}, status: http.StatusOK, deleted: true},
	}

	for i, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Окрема книга на кожен випадок: успішне видалення не впливає на інші.
			id := e.addBook(fmt.Sprintf("Disposable %d", i), fmt.Sprintf("978000000%04d", i), e.cat, e.author, "available")
			tc.req.method, tc.req.path = http.MethodDelete, fmt.Sprintf("/web/books/%d", id)

			rec := e.do(tc.req)
			if rec.Code != tc.status {
				t.Fatalf("статус = %d, очікувався %d: %s", rec.Code, tc.status, rec.Body)
			}

			var count int64
			e.db.Model(&model.Book{}).Where("id = ?", id).Count(&count)
			if tc.deleted {
				if rec.Body.Len() != 0 {
					t.Errorf("тіло має бути порожнім (HTMX прибирає рядок), отримано %q", rec.Body)
				}
				if count != 0 {
					t.Error("книга лишилась у БД")
				}
			} else if count != 1 {
				t.Error("заборонений запит видалив книгу")
			}
		})
	}

	t.Run("анонім отримує HX-Redirect на /login", func(t *testing.T) {
		rec := e.do(request{method: http.MethodDelete, path: fmt.Sprintf("/web/books/%d", e.books["Dune"]), header: htmx})
		if got := rec.Header().Get("HX-Redirect"); got != "/login" {
			t.Errorf("HX-Redirect = %q", got)
		}
	})
	t.Run("неіснуюча книга -> 404", func(t *testing.T) {
		rec := e.do(request{method: http.MethodDelete, path: "/web/books/999999", header: htmx, role: model.RoleLibrarian})
		if rec.Code != http.StatusNotFound {
			t.Errorf("статус = %d", rec.Code)
		}
	})
}

// Форми запису каталогу закриті так само, як і hx-delete.
func TestBookForms_RBAC(t *testing.T) {
	e := newEnv(t)

	tests := []struct {
		name   string
		req    request
		status int
	}{
		{"анонім на /web/books/new -> переадресація на /login", request{method: http.MethodGet, path: "/web/books/new"}, http.StatusSeeOther},
		{"reader на /web/books/new -> 403", request{method: http.MethodGet, path: "/web/books/new", role: model.RoleReader}, http.StatusForbidden},
		{"librarian на /web/books/new -> 200", request{method: http.MethodGet, path: "/web/books/new", role: model.RoleLibrarian}, http.StatusOK},
		{"reader POST /web/books -> 403", request{method: http.MethodPost, path: "/web/books", role: model.RoleReader}, http.StatusForbidden},
		{"librarian редагує -> 200", request{method: http.MethodGet, path: fmt.Sprintf("/web/books/%d/edit", e.books["Dune"]), role: model.RoleLibrarian}, http.StatusOK},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if rec := e.do(tc.req); rec.Code != tc.status {
				t.Fatalf("статус = %d, очікувався %d: %s", rec.Code, tc.status, rec.Body)
			}
		})
	}
}

// Створення книги через HTML-форму: успіх -> 303 на каталог; помилка -> 422 з формою.
func TestCreateBookForm(t *testing.T) {
	e := newEnv(t)
	post := func(form url.Values) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodPost, "/web/books", strings.NewReader(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		u := e.users[model.RoleLibrarian]
		token, _ := auth.GenerateToken(u.ID, u.Role)
		req.AddCookie(&http.Cookie{Name: middleware.AuthCookieName, Value: token})
		rec := httptest.NewRecorder()
		e.router.ServeHTTP(rec, req)
		return rec
	}

	rec := post(url.Values{
		"title": {"Refactoring"}, "isbn": {"9780134757599"}, "category_id": {fmt.Sprint(e.cat)},
		"author_ids": {fmt.Sprint(e.author)}, "published_year": {"2018"},
	})
	if rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != "/catalog?msg=created" {
		t.Fatalf("створення: %d %q %s", rec.Code, rec.Header().Get("Location"), rec.Body)
	}

	rec = post(url.Values{"title": {"<b>No ISBN</b>"}})
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("невалідна форма: статус %d", rec.Code)
	}
	// Введене значення повертається у форму ЕКРАНОВАНИМ.
	checkBody(t, rec.Body.String(), []string{"isbn is required", `value="&lt;b&gt;No ISBN&lt;/b&gt;"`}, []string{"<b>No ISBN</b>"})
}

// Автоекранування html/template: дані з БД не виконуються як HTML.
func TestAutoescaping(t *testing.T) {
	e := newEnv(t)
	e.addBook(`<script>alert("xss")</script>`, "9789999999999", e.cat, e.author)

	for _, path := range []string{"/catalog", "/web/books?q=script"} {
		rec := e.do(request{method: http.MethodGet, path: path, header: htmx})
		checkBody(t, rec.Body.String(),
			[]string{"&lt;script&gt;alert(&#34;xss&#34;)&lt;/script&gt;"},
			[]string{`<script>alert("xss")</script>`})
	}
}

// Вхід через форму: JWT лише в cookie HttpOnly + SameSite=Strict, не в тілі.
func TestLoginSetsHttpOnlyCookie(t *testing.T) {
	e := newEnv(t)
	login := func(password string) *httptest.ResponseRecorder {
		form := url.Values{"email": {"librarian@libratrack.com"}, "password": {password}}
		req := httptest.NewRequest(http.MethodPost, "/login", strings.NewReader(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		rec := httptest.NewRecorder()
		e.router.ServeHTTP(rec, req)
		return rec
	}

	rec := login(testPassword)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("статус = %d: %s", rec.Code, rec.Body)
	}
	cookies := rec.Result().Cookies()
	if len(cookies) != 1 || cookies[0].Name != middleware.AuthCookieName {
		t.Fatalf("cookies = %v", cookies)
	}
	c := cookies[0]
	if !c.HttpOnly || c.SameSite != http.SameSiteStrictMode || c.Path != "/" || c.Value == "" {
		t.Errorf("небезпечні атрибути cookie: %+v", c)
	}
	if _, err := auth.ParseToken(c.Value); err != nil {
		t.Errorf("у cookie недійсний JWT: %v", err)
	}
	if strings.Contains(rec.Body.String(), c.Value) {
		t.Error("токен потрапив у тіло відповіді")
	}

	if rec := login("wrong-password"); rec.Code != http.StatusUnauthorized || len(rec.Result().Cookies()) != 0 {
		t.Errorf("неправильний пароль: статус %d, cookies %v", rec.Code, rec.Result().Cookies())
	}
}

// Статика роздається з embed.FS (fs.Sub прибрав префікс static/).
func TestStaticAssetsEmbedded(t *testing.T) {
	e := newEnv(t)

	tests := []struct {
		path        string
		status      int
		contentType string
	}{
		{"/static/css/app.css", http.StatusOK, "text/css"},
		{"/static/js/htmx.min.js", http.StatusOK, "text/javascript"},
		{"/static/js/app.js", http.StatusOK, "text/javascript"},
		{"/static/", http.StatusNotFound, ""},    // без переліку каталогу
		{"/static/js/", http.StatusNotFound, ""}, // без переліку каталогу
		{"/static/missing.css", http.StatusNotFound, ""},
	}
	for _, tc := range tests {
		t.Run(tc.path, func(t *testing.T) {
			rec := e.do(request{method: http.MethodGet, path: tc.path})
			if rec.Code != tc.status {
				t.Fatalf("статус = %d, очікувався %d", rec.Code, tc.status)
			}
			if tc.contentType != "" && !strings.HasPrefix(rec.Header().Get("Content-Type"), tc.contentType) {
				t.Errorf("Content-Type = %q", rec.Header().Get("Content-Type"))
			}
		})
	}
}
