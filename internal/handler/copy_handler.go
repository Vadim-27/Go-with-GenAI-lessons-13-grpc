package handler

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	"libratrack/internal/model"
	"libratrack/internal/repository"
)

// CopyHandler serves the copies of a book: /books/{id}/copies and
// /books/{id}/availability.
type CopyHandler struct {
	repo repository.CopyRepository
}

// NewCopyHandler returns a handler backed by repo.
func NewCopyHandler(repo repository.CopyRepository) *CopyHandler {
	return &CopyHandler{repo: repo}
}

// RegisterPublic підключає маршрути читання (доступні без токена).
func (h *CopyHandler) RegisterPublic(r *gin.RouterGroup) {
	r.GET("/books/:id/copies", h.ListCopies)
	r.GET("/books/:id/availability", h.GetAvailability)
}

// RegisterStaff підключає маршрути запису (librarian / admin).
func (h *CopyHandler) RegisterStaff(r *gin.RouterGroup) {
	r.POST("/books/:id/copies", h.CreateCopy)
}

// CreateCopy answers POST /books/{id}/copies.
func (h *CopyHandler) CreateCopy(c *gin.Context) {
	bookID, ok := parseID(c)
	if !ok {
		return
	}
	var in model.CopyInput
	if !bindInput(c, &in) {
		return
	}

	bookCopy := in.ToCopy(bookID)
	if err := h.repo.Create(c.Request.Context(), &bookCopy); err != nil {
		respondCopyError(c, err)
		return
	}
	c.JSON(http.StatusCreated, bookCopy)
}

// ListCopies answers GET /books/{id}/copies.
func (h *CopyHandler) ListCopies(c *gin.Context) {
	bookID, ok := parseID(c)
	if !ok {
		return
	}

	copies, err := h.repo.ListByBook(c.Request.Context(), bookID)
	if err != nil {
		respondCopyError(c, err)
		return
	}
	c.JSON(http.StatusOK, copies)
}

// GetAvailability answers GET /books/{id}/availability.
func (h *CopyHandler) GetAvailability(c *gin.Context) {
	bookID, ok := parseID(c)
	if !ok {
		return
	}

	availability, err := h.repo.Availability(c.Request.Context(), bookID)
	if err != nil {
		respondCopyError(c, err)
		return
	}
	c.JSON(http.StatusOK, availability)
}

// respondCopyError is respondRepoError for copy routes. The only foreign key
// of a copy is its book, so an invalid reference means "book not found".
func respondCopyError(c *gin.Context, err error) {
	if errors.Is(err, repository.ErrNotFound) || errors.Is(err, repository.ErrInvalidReference) {
		respondError(c, http.StatusNotFound, model.CodeNotFound, "Book not found")
		return
	}
	respondRepoError(c, err, "Copy")
}
