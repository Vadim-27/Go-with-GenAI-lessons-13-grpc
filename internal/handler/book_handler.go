package handler

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"libratrack/internal/model"
	"libratrack/internal/repository"
)

// BookHandler serves the /books resource. It depends on the repository
// interface, not on GORM.
type BookHandler struct {
	repo repository.BookRepository
}

// NewBookHandler returns a handler backed by repo.
func NewBookHandler(repo repository.BookRepository) *BookHandler {
	return &BookHandler{repo: repo}
}

// RegisterPublic підключає маршрути читання (доступні без токена).
func (h *BookHandler) RegisterPublic(r *gin.RouterGroup) {
	r.GET("/books", h.ListBooks)
	r.GET("/books/:id", h.GetBook)
}

// RegisterStaff підключає маршрути запису (librarian / admin).
func (h *BookHandler) RegisterStaff(r *gin.RouterGroup) {
	r.POST("/books", h.CreateBook)
	r.PUT("/books/:id", h.UpdateBook)
	r.DELETE("/books/:id", h.DeleteBook)
}

// CreateBook answers POST /books.
func (h *BookHandler) CreateBook(c *gin.Context) {
	var in model.BookInput
	if !bindInput(c, &in) {
		return
	}

	book := in.ToBook()
	if err := h.repo.Create(c.Request.Context(), &book, in.AuthorIDs); err != nil {
		respondRepoError(c, err, "Book")
		return
	}
	c.JSON(http.StatusCreated, present(book))
}

// ListBooks answers GET /books?page=&limit=&category_id=. An empty
// category_id means "no filter".
func (h *BookHandler) ListBooks(c *gin.Context) {
	page, limit, ok := pagination(c)
	if !ok {
		return
	}

	filter := repository.BookFilter{Page: page, Limit: limit}
	if raw := c.Query("category_id"); raw != "" {
		id, err := strconv.ParseUint(raw, 10, 32)
		if err != nil || id == 0 {
			respondError(c, http.StatusBadRequest, model.CodeInvalidID, "Invalid category_id")
			return
		}
		categoryID := uint(id)
		filter.CategoryID = &categoryID
	}

	books, err := h.repo.List(c.Request.Context(), filter)
	if err != nil {
		respondRepoError(c, err, "Book")
		return
	}
	for i := range books {
		books[i] = present(books[i])
	}
	c.JSON(http.StatusOK, books)
}

// GetBook answers GET /books/{id}.
func (h *BookHandler) GetBook(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}

	book, err := h.repo.FindByID(c.Request.Context(), id)
	if err != nil {
		respondRepoError(c, err, "Book")
		return
	}
	c.JSON(http.StatusOK, present(*book))
}

// UpdateBook answers PUT /books/{id}: a full replacement of the book and its
// authors. Order of checks: id (400), body (422), existence (404).
func (h *BookHandler) UpdateBook(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	var in model.BookInput
	if !bindInput(c, &in) {
		return
	}

	book := in.ToBook()
	updated, err := h.repo.Update(c.Request.Context(), id, &book, in.AuthorIDs)
	if err != nil {
		respondRepoError(c, err, "Book")
		return
	}
	c.JSON(http.StatusOK, present(*updated))
}

// DeleteBook answers DELETE /books/{id} with 204 and no body.
func (h *BookHandler) DeleteBook(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}

	if err := h.repo.Delete(c.Request.Context(), id); err != nil {
		respondRepoError(c, err, "Book")
		return
	}
	c.Status(http.StatusNoContent)
}

// present makes sure the authors array is serialised as [] and never as null.
func present(book model.Book) model.Book {
	if book.Authors == nil {
		book.Authors = []model.Author{}
	}
	return book
}
