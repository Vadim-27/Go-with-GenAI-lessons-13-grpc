package integration_test

import (
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"libratrack/internal/auth"
	"libratrack/internal/model"
)

// Табличний тест реєстрації та логіну.
func TestAuthentication_TableDriven(t *testing.T) {
	e := newEnv(t)

	tests := []struct {
		name       string
		path       string
		body       string
		wantStatus int
		check      func(t *testing.T, res map[string]any)
	}{
		{
			name:       "реєстрація: новий користувач -> 201, роль reader",
			path:       "/api/v1/auth/register",
			body:       `{"email":"new@libratrack.com","password":"SecurePassword123!"}`,
			wantStatus: http.StatusCreated,
			check: func(t *testing.T, res map[string]any) {
				assert.Equal(t, model.RoleReader, res["role"])
				assert.NotContains(t, res, "password_hash", "хеш не має потрапляти у відповідь")
				assert.NotContains(t, res, "password")
			},
		},
		{
			name:       "реєстрація: дубль email -> 422",
			path:       "/api/v1/auth/register",
			body:       `{"email":"` + readerEmail + `","password":"SecurePassword123!"}`,
			wantStatus: http.StatusUnprocessableEntity,
			check: func(t *testing.T, res map[string]any) {
				_, msg := errInfo(res)
				assert.Equal(t, "email already registered", msg)
			},
		},
		{
			name:       "реєстрація: дубль email в іншому регістрі -> 422",
			path:       "/api/v1/auth/register",
			body:       `{"email":"READER@LibraTrack.com","password":"SecurePassword123!"}`,
			wantStatus: http.StatusUnprocessableEntity,
		},
		{
			name:       "реєстрація: Mass Assignment (role=admin) -> 422",
			path:       "/api/v1/auth/register",
			body:       `{"email":"evil@libratrack.com","password":"SecurePassword123!","role":"admin"}`,
			wantStatus: http.StatusUnprocessableEntity,
		},
		{
			name:       "реєстрація: короткий пароль -> 422",
			path:       "/api/v1/auth/register",
			body:       `{"email":"short@libratrack.com","password":"123"}`,
			wantStatus: http.StatusUnprocessableEntity,
		},
		{
			name:       "логін: правильні дані -> 200 і JWT",
			path:       "/api/v1/auth/login",
			body:       `{"email":"` + readerEmail + `","password":"` + testPassword + `"}`,
			wantStatus: http.StatusOK,
			check: func(t *testing.T, res map[string]any) {
				token, _ := res["token"].(string)
				require.NotEmpty(t, token)
				claims, err := auth.ParseToken(token)
				require.NoError(t, err)
				assert.Equal(t, model.RoleReader, claims.Role)
				assert.WithinDuration(t, time.Now().Add(24*time.Hour), claims.ExpiresAt.Time, time.Minute)
				user, _ := res["user"].(map[string]any)
				assert.NotContains(t, user, "password_hash")
			},
		},
		{
			name:       "логін: email у іншому регістрі -> 200",
			path:       "/api/v1/auth/login",
			body:       `{"email":"READER@libratrack.com","password":"` + testPassword + `"}`,
			wantStatus: http.StatusOK,
		},
		{
			name:       "логін: неправильний пароль -> 401 invalid credentials",
			path:       "/api/v1/auth/login",
			body:       `{"email":"` + readerEmail + `","password":"WrongPassword!"}`,
			wantStatus: http.StatusUnauthorized,
			check: func(t *testing.T, res map[string]any) {
				_, msg := errInfo(res)
				assert.Equal(t, "invalid credentials", msg)
			},
		},
		{
			name:       "логін: невідомий email -> те саме 401 (без User Enumeration)",
			path:       "/api/v1/auth/login",
			body:       `{"email":"unknown@libratrack.com","password":"AnyPassword!"}`,
			wantStatus: http.StatusUnauthorized,
			check: func(t *testing.T, res map[string]any) {
				_, msg := errInfo(res)
				assert.Equal(t, "invalid credentials", msg)
			},
		},
		{
			name:       "логін: порожнє тіло -> 422",
			path:       "/api/v1/auth/login",
			body:       `{}`,
			wantStatus: http.StatusUnprocessableEntity,
		},
		{
			// Спроба SQL-ін'єкції через email: запит параметризований, тож це просто невідомий email.
			name:       "логін: SQL-ін'єкція в email -> 401",
			path:       "/api/v1/auth/login",
			body:       `{"email":"' OR '1'='1","password":"x"}`,
			wantStatus: http.StatusUnauthorized,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			code, res := e.do(http.MethodPost, tt.path, "", tt.body)
			assert.Equal(t, tt.wantStatus, code, "тіло: %v", res)
			if tt.check != nil {
				tt.check(t, res)
			}
		})
	}
}

// Зареєстрований користувач має мати роль reader у БД і хешований (bcrypt) пароль.
func TestRegister_StoresBcryptHashAndReaderRole(t *testing.T) {
	e := newEnv(t)
	code, _ := e.do(http.MethodPost, "/api/v1/auth/register", "",
		`{"email":"stored@libratrack.com","password":"SecurePassword123!"}`)
	require.Equal(t, http.StatusCreated, code)

	var u model.User
	require.NoError(t, e.db.Where("email = ?", "stored@libratrack.com").First(&u).Error)
	assert.Equal(t, model.RoleReader, u.Role)
	assert.NotContains(t, u.PasswordHash, "SecurePassword123!", "пароль не має зберігатися у відкритому вигляді")
	assert.True(t, strings.HasPrefix(u.PasswordHash, "$2"), "очікували bcrypt-хеш, отримали %q", u.PasswordHash)
	assert.True(t, auth.CheckPassword(u.PasswordHash, "SecurePassword123!"))
}

// Захищений маршрут: різні варіанти поганого токена дають 401.
func TestProtectedRoute_Rejects(t *testing.T) {
	e := newEnv(t)
	reader := e.users[model.RoleReader]

	tests := []struct {
		name    string
		header  string
		wantMsg string
	}{
		{"без заголовка", "", "missing or malformed token"},
		{"схема не Bearer", "Basic abc", "missing or malformed token"},
		{"сміття замість токена", "Bearer garbage", "invalid or expired token"},
		{"прострочений токен", "Bearer " + signedToken(t, jwt.SigningMethodHS256, []byte(testSecret), reader.ID, "reader", -15*time.Minute), "invalid or expired token"},
		{"підпис чужим секретом", "Bearer " + signedToken(t, jwt.SigningMethodHS256, []byte("another-secret-that-is-long-enough-123"), reader.ID, "reader", time.Hour), "invalid or expired token"},
		{"підміна алгоритму (none)", "Bearer " + signedToken(t, jwt.SigningMethodNone, jwt.UnsafeAllowNoneSignatureType, reader.ID, "admin", time.Hour), "invalid or expired token"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			code, res := e.doHeader(http.MethodGet, "/api/v1/me/loans", tt.header)
			assert.Equal(t, http.StatusUnauthorized, code)
			c, msg := errInfo(res)
			assert.Equal(t, model.CodeUnauthenticated, c)
			assert.Equal(t, tt.wantMsg, msg)
		})
	}
}

// Профіль: користувач змінює лише ім'я, а роль/email через PUT /me змінити не може.
func TestProfile_UpdateOnlyFullName(t *testing.T) {
	e := newEnv(t)
	tok := e.token(model.RoleReader)

	code, res := e.do(http.MethodPut, "/api/v1/me", tok, `{"full_name":"Ivan Petrenko"}`)
	require.Equal(t, http.StatusOK, code)
	assert.Equal(t, "Ivan Petrenko", res["full_name"])
	assert.Equal(t, model.RoleReader, res["role"])

	code, _ = e.do(http.MethodPut, "/api/v1/me", tok, `{"full_name":"X","role":"admin"}`)
	assert.Equal(t, http.StatusUnprocessableEntity, code, "поле role у профілі має відхилятися")

	var u model.User
	require.NoError(t, e.db.First(&u, e.users[model.RoleReader].ID).Error)
	assert.Equal(t, model.RoleReader, u.Role)
}

// Відповіді з токеном не можна кешувати.
func TestLogin_ResponseIsNotCacheable(t *testing.T) {
	e := newEnv(t)
	rec := e.raw(http.MethodPost, "/api/v1/auth/login", `{"email":"`+readerEmail+`","password":"`+testPassword+`"}`)
	require.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "no-store", rec.Header().Get("Cache-Control"))
}

// Занадто велике тіло (понад 1 МБ) відхиляється, а не читається в пам'ять.
func TestRequestBody_TooLargeIsRejected(t *testing.T) {
	e := newEnv(t)
	huge := `{"email":"x@libratrack.com","password":"SecurePassword123!","full_name":"` + strings.Repeat("a", 2<<20) + `"}`
	code, _ := e.do(http.MethodPost, "/api/v1/auth/register", "", huge)
	assert.Equal(t, http.StatusUnprocessableEntity, code)

	var n int64
	require.NoError(t, e.db.Model(&model.User{}).Where("email = ?", "x@libratrack.com").Count(&n).Error)
	assert.Zero(t, n)
}
