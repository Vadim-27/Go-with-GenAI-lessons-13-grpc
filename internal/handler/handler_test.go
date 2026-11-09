package handler_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"libratrack/internal/auth"
	"libratrack/internal/model"
	"libratrack/internal/testutil"
)

// schemaName is unique to this package, so parallel test packages do not collide.
const schemaName = "t_handler"

// api is a router on top of a clean, migrated database.
type api struct {
	t      *testing.T
	router http.Handler
	token  string // JWT бібліотекаря: усі записи каталогу потребують ролі staff
}

func newAPI(t *testing.T) *api {
	t.Helper()
	t.Setenv("JWT_SECRET", "handler-test-secret-at-least-32-characters")

	db := testutil.NewDB(t, schemaName)
	// Каталог змінює лише staff, тож створюємо бібліотекаря і видаємо йому токен.
	staff := model.User{Email: "staff@libratrack.com", PasswordHash: "x", Role: model.RoleLibrarian}
	if err := db.Create(&staff).Error; err != nil {
		t.Fatal(err)
	}
	token, err := auth.GenerateToken(staff.ID, staff.Role)
	if err != nil {
		t.Fatal(err)
	}
	return &api{t: t, router: testutil.NewRouter(t, db), token: token}
}

// call sends a request and returns the status code and the decoded JSON body.
func (a *api) call(method, path, body string) (int, map[string]any) {
	a.t.Helper()

	req := httptest.NewRequest(method, path, strings.NewReader(body))
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	if a.token != "" {
		req.Header.Set("Authorization", "Bearer "+a.token)
	}
	rec := httptest.NewRecorder()
	a.router.ServeHTTP(rec, req)

	var decoded map[string]any
	if rec.Body.Len() > 0 {
		var generic any
		if err := json.Unmarshal(rec.Body.Bytes(), &generic); err != nil {
			a.t.Fatalf("%s %s: response is not JSON: %v (%s)", method, path, err, rec.Body.String())
		}
		if m, ok := generic.(map[string]any); ok {
			decoded = m
		} else {
			decoded = map[string]any{"list": generic}
		}
	}
	return rec.Code, decoded
}

func (a *api) mustCreate(path, body string) map[string]any {
	a.t.Helper()
	code, res := a.call(http.MethodPost, path, body)
	if code != http.StatusCreated {
		a.t.Fatalf("POST %s = %d, want 201: %v", path, code, res)
	}
	return res
}

// id returns the numeric id of a created entity as an integer.
func id(res map[string]any) int { return int(res["id"].(float64)) }

func errorCode(res map[string]any) string {
	e, _ := res["error"].(map[string]any)
	code, _ := e["code"].(string)
	return code
}

// seed creates one category and one author and returns their ids.
func (a *api) seed() (categoryID, authorID int) {
	a.t.Helper()
	categoryID = id(a.mustCreate("/api/v1/categories", `{"name":"Programming"}`))
	authorID = id(a.mustCreate("/api/v1/authors", `{"full_name":"Alan Donovan"}`))
	return categoryID, authorID
}

func bookBody(isbn string, categoryID int, authorIDs ...int) string {
	return fmt.Sprintf(`{"title":"Book %s","isbn":"%s","category_id":%d,"author_ids":%s}`,
		isbn, isbn, categoryID, toJSON(authorIDs))
}

func toJSON(v any) string {
	raw, _ := json.Marshal(v)
	return string(raw)
}

func TestHealth(t *testing.T) {
	a := newAPI(t)

	code, res := a.call(http.MethodGet, "/health", "")

	if code != http.StatusOK || res["status"] != "ok" {
		t.Fatalf("GET /health = %d %v", code, res)
	}
}

func TestCategories_CreateGetListAndDuplicate(t *testing.T) {
	a := newAPI(t)

	created := a.mustCreate("/api/v1/categories", `{"name":"Fantasy"}`)
	path := fmt.Sprintf("/api/v1/categories/%d", id(created))

	if code, res := a.call(http.MethodGet, path, ""); code != http.StatusOK || res["name"] != "Fantasy" {
		t.Errorf("GET category = %d %v", code, res)
	}
	if code, res := a.call(http.MethodGet, "/api/v1/categories", ""); code != http.StatusOK || len(res["list"].([]any)) != 1 {
		t.Errorf("GET categories = %d %v", code, res)
	}
	code, res := a.call(http.MethodPost, "/api/v1/categories", `{"name":"Fantasy"}`)
	if code != http.StatusConflict || errorCode(res) != "conflict" {
		t.Errorf("duplicate = %d %v, want 409 conflict", code, res)
	}
}

func TestCategories_InvalidNamesAreRejected(t *testing.T) {
	a := newAPI(t)

	for name, body := range map[string]string{
		"missing":    `{}`,
		"empty":      `{"name":""}`,
		"too short":  `{"name":"a"}`,
		"too long":   `{"name":"` + strings.Repeat("a", 41) + `"}`,
		"not json":   `not json`,
		"wrong type": `{"name":123}`,
	} {
		code, res := a.call(http.MethodPost, "/api/v1/categories", body)
		if code != http.StatusUnprocessableEntity || errorCode(res) != "validation_error" {
			t.Errorf("%s: %d %v, want 422 validation_error", name, code, res)
		}
	}
}

func TestAuthors_CreateGetAndUnknown(t *testing.T) {
	a := newAPI(t)

	created := a.mustCreate("/api/v1/authors", `{"full_name":"Frank Herbert","bio":"Writer"}`)
	path := fmt.Sprintf("/api/v1/authors/%d", id(created))

	if code, res := a.call(http.MethodGet, path, ""); code != http.StatusOK || res["full_name"] != "Frank Herbert" {
		t.Errorf("GET author = %d %v", code, res)
	}
	if code, res := a.call(http.MethodGet, "/api/v1/authors/999999", ""); code != http.StatusNotFound || errorCode(res) != "not_found" {
		t.Errorf("unknown author = %d %v, want 404 not_found", code, res)
	}
	if code, _ := a.call(http.MethodPost, "/api/v1/authors", `{}`); code != http.StatusUnprocessableEntity {
		t.Errorf("author without a name = %d, want 422", code)
	}
}

func TestBooks_CreateReturnsAuthorsAndCategory(t *testing.T) {
	a := newAPI(t)
	categoryID, authorID := a.seed()

	code, res := a.call(http.MethodPost, "/api/v1/books", bookBody("9780134190440", categoryID, authorID))

	if code != http.StatusCreated {
		t.Fatalf("POST /books = %d %v", code, res)
	}
	if len(res["authors"].([]any)) != 1 || int(res["category"].(map[string]any)["id"].(float64)) != categoryID {
		t.Errorf("response lacks relations: %v", res)
	}
	if res["publisher"] != nil || res["description"] != nil {
		t.Errorf("optional fields must be null: %v", res)
	}
}

func TestBooks_GetByID_And_Unknown(t *testing.T) {
	a := newAPI(t)
	categoryID, authorID := a.seed()
	book := a.mustCreate("/api/v1/books", bookBody("9780134190440", categoryID, authorID))

	code, res := a.call(http.MethodGet, fmt.Sprintf("/api/v1/books/%d", id(book)), "")
	if code != http.StatusOK || len(res["authors"].([]any)) != 1 {
		t.Errorf("GET book = %d %v", code, res)
	}

	code, res = a.call(http.MethodGet, "/api/v1/books/999999", "")
	if code != http.StatusNotFound || errorCode(res) != "not_found" {
		t.Errorf("unknown book = %d %v, want 404 not_found", code, res)
	}
}

func TestBooks_ValidationErrors(t *testing.T) {
	a := newAPI(t)
	categoryID, _ := a.seed()

	for name, body := range map[string]string{
		"no title":   `{"isbn":"1234567890"}`,
		"no authors": fmt.Sprintf(`{"title":"T","isbn":"1","category_id":%d,"author_ids":[]}`, categoryID),
		"bad year":   fmt.Sprintf(`{"title":"T","isbn":"1","category_id":%d,"author_ids":[1],"published_year":5}`, categoryID),
		"empty body": ``,
	} {
		code, res := a.call(http.MethodPost, "/api/v1/books", body)
		if code != http.StatusUnprocessableEntity || errorCode(res) != "validation_error" {
			t.Errorf("%s: %d %v, want 422 validation_error", name, code, res)
		}
	}
}

func TestBooks_UnknownReferences_AreInvalidReference(t *testing.T) {
	a := newAPI(t)
	categoryID, authorID := a.seed()

	code, res := a.call(http.MethodPost, "/api/v1/books", bookBody("1", categoryID, 999999))
	if code != http.StatusUnprocessableEntity || errorCode(res) != "invalid_reference" {
		t.Errorf("unknown author: %d %v, want 422 invalid_reference", code, res)
	}
	code, res = a.call(http.MethodPost, "/api/v1/books", bookBody("2", 999999, authorID))
	if code != http.StatusUnprocessableEntity || errorCode(res) != "invalid_reference" {
		t.Errorf("unknown category: %d %v, want 422 invalid_reference", code, res)
	}
	if code, _ := a.call(http.MethodGet, "/api/v1/authors/999999", ""); code != http.StatusNotFound {
		t.Errorf("an unknown author must not be created silently, GET = %d", code)
	}
}

func TestBooks_DuplicateISBN_IsConflict(t *testing.T) {
	a := newAPI(t)
	categoryID, authorID := a.seed()
	body := bookBody("9780134190440", categoryID, authorID)
	a.mustCreate("/api/v1/books", body)

	code, res := a.call(http.MethodPost, "/api/v1/books", body)

	if code != http.StatusConflict || errorCode(res) != "conflict" {
		t.Fatalf("duplicate ISBN = %d %v, want 409 conflict", code, res)
	}
}

func TestBooks_Update_ReplacesAuthors(t *testing.T) {
	a := newAPI(t)
	categoryID, authorID := a.seed()
	second := id(a.mustCreate("/api/v1/authors", `{"full_name":"Brian Kernighan"}`))
	book := a.mustCreate("/api/v1/books", bookBody("9780134190440", categoryID, authorID))
	path := fmt.Sprintf("/api/v1/books/%d", id(book))

	code, res := a.call(http.MethodPut, path, bookBody("9780134190440", categoryID, second))

	authors := res["authors"].([]any)
	if code != http.StatusOK || len(authors) != 1 || int(authors[0].(map[string]any)["id"].(float64)) != second {
		t.Fatalf("PUT = %d %v", code, res)
	}
	if code, res := a.call(http.MethodPut, "/api/v1/books/999999", bookBody("1", categoryID, authorID)); code != http.StatusNotFound || errorCode(res) != "not_found" {
		t.Errorf("PUT unknown = %d %v, want 404", code, res)
	}
}

func TestBooks_Delete_RemovesBookAndReturns204(t *testing.T) {
	a := newAPI(t)
	categoryID, authorID := a.seed()
	book := a.mustCreate("/api/v1/books", bookBody("9780134190440", categoryID, authorID))
	path := fmt.Sprintf("/api/v1/books/%d", id(book))

	if code, _ := a.call(http.MethodDelete, path, ""); code != http.StatusNoContent {
		t.Fatalf("DELETE = %d, want 204", code)
	}
	if code, _ := a.call(http.MethodGet, path, ""); code != http.StatusNotFound {
		t.Errorf("GET after DELETE = %d, want 404", code)
	}
	if code, _ := a.call(http.MethodDelete, path, ""); code != http.StatusNotFound {
		t.Errorf("second DELETE = %d, want 404", code)
	}
}

func TestBooks_List_FilterAndPagination(t *testing.T) {
	a := newAPI(t)
	categoryID, authorID := a.seed()
	other := id(a.mustCreate("/api/v1/categories", `{"name":"Fantasy"}`))
	a.mustCreate("/api/v1/books", bookBody("1111111111", categoryID, authorID))
	a.mustCreate("/api/v1/books", bookBody("2222222222", other, authorID))
	a.mustCreate("/api/v1/books", bookBody("3333333333", categoryID, authorID))

	size := func(path string) int {
		code, res := a.call(http.MethodGet, path, "")
		if code != http.StatusOK {
			t.Fatalf("GET %s = %d %v", path, code, res)
		}
		return len(res["list"].([]any))
	}

	if n := size("/api/v1/books?limit=100"); n != 3 {
		t.Errorf("all books = %d, want 3", n)
	}
	if n := size(fmt.Sprintf("/api/v1/books?category_id=%d", categoryID)); n != 2 {
		t.Errorf("filtered books = %d, want 2", n)
	}
	if n := size("/api/v1/books?limit=2&page=1"); n != 2 {
		t.Errorf("limit=2 gives %d books, want 2", n)
	}
	if n := size("/api/v1/books?limit=2&page=2"); n != 1 {
		t.Errorf("page 2 gives %d books, want 1", n)
	}
	if n := size("/api/v1/books?page=9223372036854775807"); n != 0 {
		t.Errorf("a huge page gives %d books, want 0", n)
	}
}

func TestBooks_List_EveryBookHasCategoryAndAuthors(t *testing.T) {
	a := newAPI(t)
	categoryID, authorID := a.seed()
	a.mustCreate("/api/v1/books", bookBody("1111111111", categoryID, authorID))

	_, res := a.call(http.MethodGet, "/api/v1/books", "")

	for _, item := range res["list"].([]any) {
		book := item.(map[string]any)
		if book["category"] == nil || len(book["authors"].([]any)) == 0 {
			t.Errorf("book without relations in the list: %v", book)
		}
	}
}

func TestBooks_List_InvalidQueryParameters(t *testing.T) {
	a := newAPI(t)

	for path, want := range map[string]string{
		"/api/v1/books?page=0":          "invalid_pagination",
		"/api/v1/books?limit=abc":       "invalid_pagination",
		"/api/v1/books?limit=-1":        "invalid_pagination",
		"/api/v1/books?category_id=abc": "invalid_id",
	} {
		code, res := a.call(http.MethodGet, path, "")
		if code != http.StatusBadRequest || errorCode(res) != want {
			t.Errorf("%s = %d %v, want 400 %s", path, code, res, want)
		}
	}
}

func TestCopies_CreateListAndAvailability(t *testing.T) {
	a := newAPI(t)
	categoryID, authorID := a.seed()
	book := id(a.mustCreate("/api/v1/books", bookBody("9780134190440", categoryID, authorID)))
	base := fmt.Sprintf("/api/v1/books/%d", book)

	first := a.mustCreate(base+"/copies", `{"inventory_no":"INV-1"}`)
	if first["status"] != "available" {
		t.Errorf("default status = %v, want available", first["status"])
	}
	a.mustCreate(base+"/copies", `{"inventory_no":"INV-2","status":"available"}`)
	a.mustCreate(base+"/copies", `{"inventory_no":"INV-3","status":"lost"}`)

	if code, res := a.call(http.MethodGet, base+"/copies", ""); code != http.StatusOK || len(res["list"].([]any)) != 3 {
		t.Errorf("GET copies = %d %v", code, res)
	}
	code, res := a.call(http.MethodGet, base+"/availability", "")
	if code != http.StatusOK || res["total_copies"] != float64(3) || res["available_copies"] != float64(2) || int(res["book_id"].(float64)) != book {
		t.Errorf("availability = %d %v", code, res)
	}
}

func TestCopies_Errors(t *testing.T) {
	a := newAPI(t)
	categoryID, authorID := a.seed()
	book := id(a.mustCreate("/api/v1/books", bookBody("9780134190440", categoryID, authorID)))
	base := fmt.Sprintf("/api/v1/books/%d", book)
	a.mustCreate(base+"/copies", `{"inventory_no":"INV-1"}`)

	if code, res := a.call(http.MethodPost, base+"/copies", `{"inventory_no":"INV-1"}`); code != http.StatusConflict || errorCode(res) != "conflict" {
		t.Errorf("duplicate inventory number = %d %v, want 409", code, res)
	}
	if code, _ := a.call(http.MethodPost, base+"/copies", `{"inventory_no":"INV-2","status":"broken"}`); code != http.StatusUnprocessableEntity {
		t.Errorf("invalid status = %d, want 422", code)
	}
	if code, _ := a.call(http.MethodPost, "/api/v1/books/999999/copies", `{"inventory_no":"X"}`); code != http.StatusNotFound {
		t.Errorf("copy of an unknown book = %d, want 404", code)
	}
	if code, _ := a.call(http.MethodGet, "/api/v1/books/999999/availability", ""); code != http.StatusNotFound {
		t.Errorf("availability of an unknown book = %d, want 404", code)
	}
}

func TestInvalidIDsAndUnknownRoutes(t *testing.T) {
	a := newAPI(t)

	for _, path := range []string{"/api/v1/books/abc", "/api/v1/books/-1", "/api/v1/books/0", "/api/v1/categories/x"} {
		code, res := a.call(http.MethodGet, path, "")
		if code != http.StatusBadRequest || errorCode(res) != "invalid_id" {
			t.Errorf("GET %s = %d %v, want 400 invalid_id", path, code, res)
		}
	}
	if code, res := a.call(http.MethodGet, "/nope", ""); code != http.StatusNotFound || errorCode(res) != "not_found" {
		t.Errorf("unknown route = %d %v, want 404", code, res)
	}
}

func TestCORS_PreflightIsHandled(t *testing.T) {
	a := newAPI(t)
	req := httptest.NewRequest(http.MethodOptions, "/api/v1/books", nil)
	req.Header.Set("Origin", "http://example.com")
	req.Header.Set("Access-Control-Request-Method", "POST")
	rec := httptest.NewRecorder()

	a.router.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent || rec.Header().Get("Access-Control-Allow-Origin") == "" {
		t.Fatalf("preflight = %d, headers %v", rec.Code, rec.Header())
	}
}

func TestSwaggerUI_IsServed(t *testing.T) {
	a := newAPI(t)

	for _, path := range []string{"/swagger", "/swagger/index.html", "/openapi.yaml"} {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		rec := httptest.NewRecorder()
		a.router.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Errorf("GET %s = %d, want 200", path, rec.Code)
		}
	}
}
