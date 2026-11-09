package integration_test

import (
	"fmt"
	"net/http"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"libratrack/internal/model"
)

// seedCopy створює категорію, автора, книгу та один примірник; повертає id примірника.
func seedCopy(t *testing.T, e *env) int {
	t.Helper()
	staff := e.token(model.RoleLibrarian)
	cat := mustID(t, e, "/api/v1/categories", staff, `{"name":"Programming"}`)
	author := mustID(t, e, "/api/v1/authors", staff, `{"full_name":"Alan Donovan"}`)
	book := mustID(t, e, "/api/v1/books", staff,
		fmt.Sprintf(`{"title":"Go","isbn":"9780134190440","category_id":%d,"author_ids":[%d]}`, cat, author))
	return mustID(t, e, fmt.Sprintf("/api/v1/books/%d/copies", book), staff, `{"inventory_no":"INV-1"}`)
}

// Повний цикл: видача -> повторна видача (409) -> /me/loans -> повернення -> повторне повернення (409).
func TestLoans_Lifecycle(t *testing.T) {
	e := newEnv(t)
	copyID := seedCopy(t, e)
	staff := e.token(model.RoleLibrarian)
	readerID := e.users[model.RoleReader].ID
	issue := fmt.Sprintf(`{"user_id":%d,"copy_id":%d}`, readerID, copyID)

	code, loan := e.do(http.MethodPost, "/api/v1/loans", staff, issue)
	require.Equal(t, http.StatusCreated, code, "%v", loan)
	loanID := int(loan["id"].(float64))
	assert.EqualValues(t, e.users[model.RoleLibrarian].ID, loan["issued_by"], "issued_by береться з токена")
	assert.Nil(t, loan["returned_at"])

	var cp model.Copy
	require.NoError(t, e.db.First(&cp, copyID).Error)
	assert.Equal(t, model.StatusBorrowed, cp.Status)

	code, _ = e.do(http.MethodPost, "/api/v1/loans", staff, issue)
	assert.Equal(t, http.StatusConflict, code, "виданий примірник не можна видати вдруге")

	// Читач бачить свою позику.
	code, res := e.do(http.MethodGet, "/api/v1/me/loans", e.token(model.RoleReader), "")
	require.Equal(t, http.StatusOK, code)
	assert.Len(t, res["list"], 1)

	returnPath := fmt.Sprintf("/api/v1/loans/%d/return", loanID)
	code, res = e.do(http.MethodPost, returnPath, staff, "")
	require.Equal(t, http.StatusOK, code, "%v", res)
	assert.NotNil(t, res["returned_at"])

	require.NoError(t, e.db.First(&cp, copyID).Error)
	assert.Equal(t, model.StatusAvailable, cp.Status)

	code, _ = e.do(http.MethodPost, returnPath, staff, "")
	assert.Equal(t, http.StatusConflict, code, "повторне повернення -> 409")
}

// IDOR: /me/loans показує лише позики власника токена.
func TestMyLoans_OnlyOwnLoans(t *testing.T) {
	e := newEnv(t)
	copyID := seedCopy(t, e)
	staff := e.token(model.RoleLibrarian)

	// Позика оформлена на reader.
	code, _ := e.do(http.MethodPost, "/api/v1/loans", staff,
		fmt.Sprintf(`{"user_id":%d,"copy_id":%d}`, e.users[model.RoleReader].ID, copyID))
	require.Equal(t, http.StatusCreated, code)

	// Адмін (інший користувач) не бачить чужу позику, навіть якщо підставляє user_id у query.
	code, res := e.do(http.MethodGet, fmt.Sprintf("/api/v1/me/loans?user_id=%d", e.users[model.RoleReader].ID),
		e.token(model.RoleAdmin), "")
	require.Equal(t, http.StatusOK, code)
	assert.Empty(t, res["list"], "чужі позики не повинні потрапляти у відповідь")
}

// Помилкові вхідні дані при видачі.
func TestLoans_IssueValidation(t *testing.T) {
	e := newEnv(t)
	copyID := seedCopy(t, e)
	staff := e.token(model.RoleLibrarian)
	readerID := e.users[model.RoleReader].ID

	tests := []struct {
		name string
		body string
		want int
	}{
		{"немає copy_id", fmt.Sprintf(`{"user_id":%d}`, readerID), http.StatusUnprocessableEntity},
		{"немає user_id", fmt.Sprintf(`{"copy_id":%d}`, copyID), http.StatusUnprocessableEntity},
		{"days поза межами", fmt.Sprintf(`{"user_id":%d,"copy_id":%d,"days":999}`, readerID, copyID), http.StatusUnprocessableEntity},
		{"Mass Assignment: issued_by у тілі", fmt.Sprintf(`{"user_id":%d,"copy_id":%d,"issued_by":1}`, readerID, copyID), http.StatusUnprocessableEntity},
		{"невідомий примірник", fmt.Sprintf(`{"user_id":%d,"copy_id":99999}`, readerID), http.StatusNotFound},
		{"невідомий користувач", fmt.Sprintf(`{"user_id":99999,"copy_id":%d}`, copyID), http.StatusUnprocessableEntity},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			code, res := e.do(http.MethodPost, "/api/v1/loans", staff, tt.body)
			assert.Equal(t, tt.want, code, "%v", res)
		})
	}

	// Жоден із невдалих запитів не мав змінити статус примірника.
	var cp model.Copy
	require.NoError(t, e.db.First(&cp, copyID).Error)
	assert.Equal(t, model.StatusAvailable, cp.Status)
}

// Гонка: два працівники одночасно видають той самий примірник — успіх рівно один.
func TestLoans_ConcurrentIssueOnlyOnce(t *testing.T) {
	e := newEnv(t)
	copyID := seedCopy(t, e)
	staff := e.token(model.RoleLibrarian)
	body := fmt.Sprintf(`{"user_id":%d,"copy_id":%d}`, e.users[model.RoleReader].ID, copyID)

	const workers = 8
	codes := make(chan int, workers)
	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			code, _ := e.do(http.MethodPost, "/api/v1/loans", staff, body)
			codes <- code
		}()
	}
	wg.Wait()
	close(codes)

	created, conflicts := 0, 0
	for code := range codes {
		switch code {
		case http.StatusCreated:
			created++
		case http.StatusConflict:
			conflicts++
		}
	}
	assert.Equal(t, 1, created, "примірник мав бути виданий рівно один раз")
	assert.Equal(t, workers-1, conflicts)
}
