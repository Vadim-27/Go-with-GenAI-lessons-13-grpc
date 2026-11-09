// Package integration_test містить інтеграційні тести безпеки (auth + RBAC).
// Тести працюють зі СПРАВЖНІМ роутером застосунку та реальною PostgreSQL
// (TEST_DATABASE_URL), без моків: так перевіряється код, який піде в продакшн.
package integration_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"gorm.io/gorm"

	"libratrack/internal/auth"
	"libratrack/internal/model"
	"libratrack/internal/testutil"
)

// Окрема схема БД для цього пакета: пакети тестів Go запускаються паралельно.
const schemaName = "t_integration"

const (
	testSecret    = "integration-test-secret-at-least-32-chars"
	testPassword  = "CorrectPassword123!"
	readerEmail   = "reader@libratrack.com"
	librarianMail = "librarian@libratrack.com"
	adminEmail    = "admin@libratrack.com"
)

// env — роутер, БД і готові користувачі трьох ролей.
type env struct {
	t      *testing.T
	db     *gorm.DB
	router http.Handler
	users  map[string]model.User // за роллю
}

// newEnv піднімає чисту БД, роутер і по одному користувачу кожної ролі
// (пароль у всіх — testPassword).
func newEnv(t *testing.T) *env {
	t.Helper()
	t.Setenv("JWT_SECRET", testSecret)

	db := testutil.NewDB(t, schemaName)
	e := &env{t: t, db: db, router: testutil.NewRouter(t, db), users: map[string]model.User{}}

	hash, err := auth.HashPassword(testPassword)
	if err != nil {
		t.Fatal(err)
	}
	for role, email := range map[string]string{
		model.RoleReader:    readerEmail,
		model.RoleLibrarian: librarianMail,
		model.RoleAdmin:     adminEmail,
	} {
		u := model.User{Email: email, PasswordHash: hash, Role: role}
		if err := db.Create(&u).Error; err != nil {
			t.Fatal(err)
		}
		e.users[role] = u
	}
	return e
}

// token видає дійсний токен користувачу з указаною роллю.
func (e *env) token(role string) string {
	e.t.Helper()
	u := e.users[role]
	tok, err := auth.GenerateToken(u.ID, u.Role)
	if err != nil {
		e.t.Fatal(err)
	}
	return tok
}

// do надсилає запит (token порожній = без Authorization) і повертає код та розібране тіло.
func (e *env) do(method, path, token, body string) (int, map[string]any) {
	e.t.Helper()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	rec := httptest.NewRecorder()
	e.router.ServeHTTP(rec, req)

	var decoded map[string]any
	if rec.Body.Len() > 0 {
		var generic any
		if err := json.Unmarshal(rec.Body.Bytes(), &generic); err != nil {
			e.t.Fatalf("%s %s: тіло не JSON: %v (%s)", method, path, err, rec.Body)
		}
		if m, ok := generic.(map[string]any); ok {
			decoded = m
		} else {
			decoded = map[string]any{"list": generic}
		}
	}
	return rec.Code, decoded
}

// errInfo витягує code і message з {"error": {...}}.
func errInfo(res map[string]any) (code, message string) {
	obj, _ := res["error"].(map[string]any)
	code, _ = obj["code"].(string)
	message, _ = obj["message"].(string)
	return code, message
}

// signedToken підписує довільні claims тим самим секретом (для сценаріїв з простроченням тощо).
func signedToken(t *testing.T, method jwt.SigningMethod, key any, userID uint, role string, ttl time.Duration) string {
	t.Helper()
	claims := auth.Claims{UserID: userID, Role: role, RegisteredClaims: jwt.RegisteredClaims{
		ExpiresAt: jwt.NewNumericDate(time.Now().Add(ttl)),
	}}
	s, err := jwt.NewWithClaims(method, claims).SignedString(key)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

// doHeader надсилає запит із довільним значенням заголовка Authorization (як є).
func (e *env) doHeader(method, path, authorization string) (int, map[string]any) {
	e.t.Helper()
	req := httptest.NewRequest(method, path, nil)
	if authorization != "" {
		req.Header.Set("Authorization", authorization)
	}
	rec := httptest.NewRecorder()
	e.router.ServeHTTP(rec, req)

	var decoded map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &decoded)
	return rec.Code, decoded
}

// raw повертає сирий httptest.ResponseRecorder (для перевірки заголовків відповіді).
func (e *env) raw(method, path, body string) *httptest.ResponseRecorder {
	e.t.Helper()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	e.router.ServeHTTP(rec, req)
	return rec
}
