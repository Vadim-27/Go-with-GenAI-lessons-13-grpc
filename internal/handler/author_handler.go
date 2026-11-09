package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"libratrack/internal/model"
	"libratrack/internal/repository"
)

// AuthorHandler serves the /authors resource.
type AuthorHandler struct {
	repo repository.AuthorRepository
}

// NewAuthorHandler returns a handler backed by repo.
func NewAuthorHandler(repo repository.AuthorRepository) *AuthorHandler {
	return &AuthorHandler{repo: repo}
}

// RegisterPublic підключає маршрути читання (доступні без токена).
func (h *AuthorHandler) RegisterPublic(r *gin.RouterGroup) {
	r.GET("/authors", h.ListAuthors)
	r.GET("/authors/:id", h.GetAuthor)
}

// RegisterStaff підключає маршрути запису (librarian / admin).
func (h *AuthorHandler) RegisterStaff(r *gin.RouterGroup) {
	r.POST("/authors", h.CreateAuthor)
}

// CreateAuthor answers POST /authors.
func (h *AuthorHandler) CreateAuthor(c *gin.Context) {
	var in model.AuthorInput
	if !bindInput(c, &in) {
		return
	}

	author := in.ToAuthor()
	if err := h.repo.Create(c.Request.Context(), &author); err != nil {
		respondRepoError(c, err, "Author")
		return
	}
	c.JSON(http.StatusCreated, author)
}

// ListAuthors answers GET /authors?page=&limit=.
func (h *AuthorHandler) ListAuthors(c *gin.Context) {
	page, limit, ok := pagination(c)
	if !ok {
		return
	}

	authors, err := h.repo.List(c.Request.Context(), page, limit)
	if err != nil {
		respondRepoError(c, err, "Author")
		return
	}
	c.JSON(http.StatusOK, authors)
}

// GetAuthor answers GET /authors/{id}.
func (h *AuthorHandler) GetAuthor(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}

	author, err := h.repo.FindByID(c.Request.Context(), id)
	if err != nil {
		respondRepoError(c, err, "Author")
		return
	}
	c.JSON(http.StatusOK, author)
}
