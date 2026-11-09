// Package handler contains the HTTP layer: routing, request parsing and
// the mapping of repository errors to HTTP responses.
package handler

import (
	"encoding/json"
	"errors"
	"io"
	"log"
	"math"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"libratrack/internal/model"
	"libratrack/internal/repository"
)

// maxBodyBytes — максимальний розмір тіла JSON-запиту (1 МБ).
const maxBodyBytes = 1 << 20

// Pagination parameters of list endpoints.
const (
	defaultPage  = 1
	defaultLimit = 10
	maxLimit     = 100
)

// respondError writes the JSON error envelope with the given HTTP status.
func respondError(c *gin.Context, status int, code, message string) {
	c.JSON(status, model.NewErrorResponse(code, message))
}

// respondRepoError maps an error from the repository layer to an HTTP
// response. resource is the human-readable entity name used in messages.
func respondRepoError(c *gin.Context, err error, resource string) {
	switch {
	case errors.Is(err, repository.ErrNotFound):
		respondError(c, http.StatusNotFound, model.CodeNotFound, resource+" not found")
	case errors.Is(err, repository.ErrConflict):
		respondError(c, http.StatusConflict, model.CodeConflict, resource+" already exists")
	case errors.Is(err, repository.ErrInvalidReference):
		respondError(c, http.StatusUnprocessableEntity, model.CodeInvalidReference,
			"A referenced resource (author, category, user) does not exist")
	case errors.Is(err, repository.ErrCopyUnavailable):
		respondError(c, http.StatusConflict, model.CodeConflict, "Copy is not available for loan")
	case errors.Is(err, repository.ErrAlreadyReturned):
		respondError(c, http.StatusConflict, model.CodeConflict, "Loan is already returned")
	case errors.Is(err, model.ErrValidation):
		respondError(c, http.StatusUnprocessableEntity, model.CodeValidationError, err.Error())
	default:
		// Unknown error: details go to the log, the client gets no internals.
		log.Printf("internal error: %v", err)
		respondError(c, http.StatusInternalServerError, model.CodeInternalServerError, "Internal server error")
	}
}

// validatable is implemented by every request input type.
type validatable interface {
	Validate() error
}

// bindInput decodes and validates the request body. When it returns false
// the 422 response is already written and the caller must stop.
func bindInput(c *gin.Context, in validatable) bool {
	if err := decodeBody(c, in); err != nil {
		// The decoder error is an internal detail (types, offsets): not shown to the client.
		respondError(c, http.StatusUnprocessableEntity, model.CodeValidationError, "Invalid request body")
		return false
	}
	if err := in.Validate(); err != nil {
		respondError(c, http.StatusUnprocessableEntity, model.CodeValidationError, err.Error())
		return false
	}
	return true
}

// decodeBody decodes the body into dst and requires that nothing follows the
// JSON object: `{...} garbage` is not valid JSON.
func decodeBody(c *gin.Context, dst any) error {
	// Обмеження розміру тіла: інакше величезний JSON з'їсть пам'ять (DoS).
	// Перевищення дає помилку читання -> 422 "Invalid request body".
	body := http.MaxBytesReader(c.Writer, c.Request.Body, maxBodyBytes)
	dec := json.NewDecoder(body)
	// Невідомі поля ({"role":"admin"} у RegisterRequest) — помилка, а не тихе
	// ігнорування: це захист від Mass Assignment.
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		return err
	}
	if _, err := dec.Token(); !errors.Is(err, io.EOF) {
		return errors.New("unexpected data after JSON body")
	}
	return nil
}

// parseID reads the {id} path parameter as a positive integer. When it
// returns false the 400 response is already written.
func parseID(c *gin.Context) (uint, bool) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil || id == 0 {
		respondError(c, http.StatusBadRequest, model.CodeInvalidID, "Invalid id")
		return 0, false
	}
	return uint(id), true
}

// pagination reads page and limit from the query string. When it returns
// false the 400 response is already written.
func pagination(c *gin.Context) (page, limit int, ok bool) {
	page, ok = queryPositiveInt(c, "page", defaultPage)
	if !ok {
		respondError(c, http.StatusBadRequest, model.CodeInvalidPagination, "Invalid page")
		return 0, 0, false
	}
	limit, ok = queryPositiveInt(c, "limit", defaultLimit)
	if !ok {
		respondError(c, http.StatusBadRequest, model.CodeInvalidPagination, "Invalid limit")
		return 0, 0, false
	}
	if limit > maxLimit {
		limit = maxLimit
	}
	return page, limit, true
}

// queryPositiveInt reads an integer query parameter >= 1. A missing
// parameter gives def. A non-integer, zero or negative value gives false.
// A huge positive value is clamped to math.MaxInt so it can never panic.
func queryPositiveInt(c *gin.Context, key string, def int) (int, bool) {
	raw, present := c.GetQuery(key)
	if !present {
		return def, true
	}

	n, err := strconv.ParseInt(raw, 10, 64)
	if errors.Is(err, strconv.ErrRange) && !strings.HasPrefix(raw, "-") {
		return math.MaxInt, true
	}
	if err != nil || n < 1 {
		return 0, false
	}
	if n > math.MaxInt {
		return math.MaxInt, true
	}
	return int(n), true
}
