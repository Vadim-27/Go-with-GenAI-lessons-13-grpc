package integration_test

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"libratrack/internal/model"
)

// Табличний тест матриці доступу: ролі x ендпоінти (див. security_design.txt).
// Для заборонених маршрутів перевіряємо код відповіді; для дозволених —
// що запит дійшов до хендлера (статус НЕ 401/403).
func TestRBAC_Matrix(t *testing.T) {
	e := newEnv(t)

	// Дані, на які посилаються тіла запитів.
	cat := mustID(t, e, "/api/v1/categories", e.token(model.RoleLibrarian), `{"name":"Programming"}`)
	author := mustID(t, e, "/api/v1/authors", e.token(model.RoleLibrarian), `{"full_name":"Alan Donovan"}`)
	isbn := 0
	newBook := func() string {
		isbn++
		return fmt.Sprintf(`{"title":"Book","isbn":"97800000%05d","category_id":%d,"author_ids":[%d]}`, isbn, cat, author)
	}

	type request struct {
		method, path string
		body         func() string
	}
	static := func(s string) func() string { return func() string { return s } }

	createBook := request{http.MethodPost, "/api/v1/books", newBook}
	listUsers := request{http.MethodGet, "/api/v1/users", static("")}
	changeRole := request{http.MethodPut, "/api/v1/users/%d/role", static(`{"role":"librarian"}`)}
	myLoans := request{http.MethodGet, "/api/v1/me/loans", static("")}
	issueLoan := request{http.MethodPost, "/api/v1/loans", static(`{"user_id":1,"copy_id":999}`)}
	returnLoan := request{http.MethodPost, "/api/v1/loans/999/return", static("")}
	publicBooks := request{http.MethodGet, "/api/v1/books", static("")}

	const (
		anon = "" // без токена
		ok   = 0  // дозволено: будь-який код, крім 401/403
	)
	tests := []struct {
		name string
		req  request
		role string // "" = анонім
		want int    // ok або конкретний код (401/403/201)
	}{
		// Публічні маршрути
		{"каталог: анонім читає книги", publicBooks, anon, http.StatusOK},

		// Захищені (будь-яка роль)
		{"/me/loans: анонім -> 401", myLoans, anon, http.StatusUnauthorized},
		{"/me/loans: reader дозволено", myLoans, model.RoleReader, http.StatusOK},
		{"/me/loans: librarian дозволено", myLoans, model.RoleLibrarian, http.StatusOK},
		{"/me/loans: admin дозволено", myLoans, model.RoleAdmin, http.StatusOK},

		// Staff: POST /books
		{"POST /books: анонім -> 401", createBook, anon, http.StatusUnauthorized},
		{"POST /books: reader -> 403", createBook, model.RoleReader, http.StatusForbidden},
		{"POST /books: librarian -> 201", createBook, model.RoleLibrarian, http.StatusCreated},
		{"POST /books: admin -> 201", createBook, model.RoleAdmin, http.StatusCreated},

		// Staff: loans
		{"POST /loans: анонім -> 401", issueLoan, anon, http.StatusUnauthorized},
		{"POST /loans: reader -> 403", issueLoan, model.RoleReader, http.StatusForbidden},
		{"POST /loans: librarian пройшов RBAC", issueLoan, model.RoleLibrarian, ok},
		{"POST /loans/:id/return: reader -> 403", returnLoan, model.RoleReader, http.StatusForbidden},
		{"POST /loans/:id/return: librarian пройшов RBAC", returnLoan, model.RoleLibrarian, ok},

		// Admin: users
		{"GET /users: анонім -> 401", listUsers, anon, http.StatusUnauthorized},
		{"GET /users: reader -> 403", listUsers, model.RoleReader, http.StatusForbidden},
		{"GET /users: librarian -> 403", listUsers, model.RoleLibrarian, http.StatusForbidden},
		{"GET /users: admin -> 200", listUsers, model.RoleAdmin, http.StatusOK},
		{"PUT /users/:id/role: reader -> 403", changeRole, model.RoleReader, http.StatusForbidden},
		{"PUT /users/:id/role: librarian -> 403", changeRole, model.RoleLibrarian, http.StatusForbidden},
		{"PUT /users/:id/role: admin пройшов RBAC", changeRole, model.RoleAdmin, http.StatusOK},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := tt.req.path
			if path == changeRole.path {
				path = fmt.Sprintf(path, e.users[model.RoleReader].ID)
			}
			token := ""
			if tt.role != anon {
				token = e.token(tt.role)
			}

			code, res := e.do(tt.req.method, path, token, tt.req.body())

			if tt.want == ok {
				assert.NotContains(t, []int{http.StatusUnauthorized, http.StatusForbidden}, code,
					"RBAC не мав зупинити запит: %v", res)
				return
			}
			assert.Equal(t, tt.want, code, "тіло: %v", res)

			// Формат відмов: 401 -> unauthenticated, 403 -> forbidden.
			switch tt.want {
			case http.StatusUnauthorized:
				c, _ := errInfo(res)
				assert.Equal(t, model.CodeUnauthenticated, c)
			case http.StatusForbidden:
				c, _ := errInfo(res)
				assert.Equal(t, model.CodeForbidden, c)
			}
		})
	}
}

// Заборонений запит не повинен мати побічних ефектів: хендлер не виконується.
func TestRBAC_ForbiddenRequestDoesNotReachHandler(t *testing.T) {
	e := newEnv(t)
	code, _ := e.do(http.MethodPost, "/api/v1/categories", e.token(model.RoleReader), `{"name":"Hacked"}`)
	require.Equal(t, http.StatusForbidden, code)

	var count int64
	require.NoError(t, e.db.Table("categories").Count(&count).Error)
	assert.Zero(t, count, "категорія не мала створитися")
}

// Адмін змінює ролі; нова роль діє з наступного логіну (JWT несе роль на момент видачі).
func TestAdmin_ChangeRole(t *testing.T) {
	e := newEnv(t)
	admin := e.token(model.RoleAdmin)
	readerID := e.users[model.RoleReader].ID
	path := fmt.Sprintf("/api/v1/users/%d/role", readerID)

	tests := []struct {
		name string
		path string
		body string
		want int
	}{
		{"призначити librarian", path, `{"role":"librarian"}`, http.StatusOK},
		{"невідома роль -> 422", path, `{"role":"superuser"}`, http.StatusUnprocessableEntity},
		{"порожнє тіло -> 422", path, `{}`, http.StatusUnprocessableEntity},
		{"невідомий користувач -> 404", "/api/v1/users/99999/role", `{"role":"reader"}`, http.StatusNotFound},
		{"некоректний id -> 400", "/api/v1/users/abc/role", `{"role":"reader"}`, http.StatusBadRequest},
		{"адмін не змінює роль собі -> 422", fmt.Sprintf("/api/v1/users/%d/role", e.users[model.RoleAdmin].ID), `{"role":"reader"}`, http.StatusUnprocessableEntity},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			code, res := e.do(http.MethodPut, tt.path, admin, tt.body)
			assert.Equal(t, tt.want, code, "тіло: %v", res)
		})
	}

	// Список користувачів не містить хешів паролів.
	code, res := e.do(http.MethodGet, "/api/v1/users", admin, "")
	require.Equal(t, http.StatusOK, code)
	for _, u := range res["list"].([]any) {
		assert.NotContains(t, u.(map[string]any), "password_hash")
	}
}

// mustID створює сутність від імені користувача з токеном і повертає її id.
func mustID(t *testing.T, e *env, path, token, body string) int {
	t.Helper()
	code, res := e.do(http.MethodPost, path, token, body)
	require.Equal(t, http.StatusCreated, code, "POST %s: %v", path, res)
	return int(res["id"].(float64))
}
