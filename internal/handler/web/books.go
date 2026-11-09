package web

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"libratrack/internal/model"
	"libratrack/internal/repository"
)

// maxChoices — скільки категорій/авторів показують списки вибору у формі.
const maxChoices = 100

// bookForm — дані сторінки book_form.html (і для створення, і для редагування).
type bookForm struct {
	Action        string // URL, куди відправляється форма
	IsEdit        bool
	Title         string
	ISBN          string
	CategoryID    uint
	AuthorIDs     []uint
	Publisher     string
	PublishedYear string
	Description   string
	Categories    []model.Category
	Authors       []model.Author
	Error         string
}

// NewBookForm відповідає на GET /web/books/new.
func (h *Handler) NewBookForm(c *gin.Context) {
	h.renderForm(c, http.StatusOK, bookForm{Action: "/web/books"})
}

// EditBookForm відповідає на GET /web/books/:id/edit: форма з поточними значеннями.
func (h *Handler) EditBookForm(c *gin.Context) {
	id, ok := h.bookID(c)
	if !ok {
		return
	}
	book, err := h.repos.Books.FindByID(c.Request.Context(), id)
	if err != nil {
		h.repoError(c, err)
		return
	}

	form := bookForm{
		Action:      fmt.Sprintf("/web/books/%d", id),
		IsEdit:      true,
		Title:       book.Title,
		ISBN:        book.ISBN,
		CategoryID:  book.CategoryID,
		Publisher:   deref(book.Publisher),
		Description: deref(book.Description),
	}
	if book.PublishedYear != nil {
		form.PublishedYear = strconv.Itoa(*book.PublishedYear)
	}
	for _, a := range book.Authors {
		form.AuthorIDs = append(form.AuthorIDs, a.ID)
	}
	h.renderForm(c, http.StatusOK, form)
}

// CreateBook відповідає на POST /web/books (звичайна HTML-форма).
// Успіх — переадресація 303 на каталог (Post/Redirect/Get), помилка — та сама
// форма зі статусом 422 і збереженими значеннями.
func (h *Handler) CreateBook(c *gin.Context) {
	if !h.limitForm(c) {
		return
	}
	in, form := readBookForm(c)
	form.Action = "/web/books"
	if err := in.Validate(); err != nil {
		form.Error = validationMessage(err)
		h.renderForm(c, http.StatusUnprocessableEntity, form)
		return
	}

	book := in.ToBook()
	if err := h.repos.Books.Create(c.Request.Context(), &book, in.AuthorIDs); err != nil {
		h.formError(c, err, form)
		return
	}
	c.Redirect(http.StatusSeeOther, "/catalog?msg=created")
}

// UpdateBook відповідає на POST /web/books/:id (HTML-форми не вміють PUT).
func (h *Handler) UpdateBook(c *gin.Context) {
	id, ok := h.bookID(c)
	if !ok || !h.limitForm(c) {
		return
	}
	in, form := readBookForm(c)
	form.Action = fmt.Sprintf("/web/books/%d", id)
	form.IsEdit = true
	if err := in.Validate(); err != nil {
		form.Error = validationMessage(err)
		h.renderForm(c, http.StatusUnprocessableEntity, form)
		return
	}

	book := in.ToBook()
	if _, err := h.repos.Books.Update(c.Request.Context(), id, &book, in.AuthorIDs); err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			h.repoError(c, err)
			return
		}
		h.formError(c, err, form)
		return
	}
	c.Redirect(http.StatusSeeOther, "/catalog?msg=updated")
}

// readBookForm перетворює поля форми на model.BookInput — ту саму DTO, що й
// JSON API, тож правила валідації спільні. Повертає також значення для
// повторного показу форми. Поля поза DTO (наприклад, id чи created_at) ігноруються.
func readBookForm(c *gin.Context) (model.BookInput, bookForm) {
	form := bookForm{
		Title:         c.PostForm("title"),
		ISBN:          c.PostForm("isbn"),
		Publisher:     strings.TrimSpace(c.PostForm("publisher")),
		PublishedYear: strings.TrimSpace(c.PostForm("published_year")),
		Description:   strings.TrimSpace(c.PostForm("description")),
	}
	in := model.BookInput{Title: &form.Title, ISBN: &form.ISBN}

	if id, err := strconv.ParseUint(c.PostForm("category_id"), 10, 32); err == nil && id > 0 {
		form.CategoryID = uint(id)
		in.CategoryID = &form.CategoryID
	}
	for _, raw := range c.PostFormArray("author_ids") {
		// Нечислове значення стає 0, і Validate повідомить "positive ids".
		id, _ := strconv.ParseUint(raw, 10, 32)
		form.AuthorIDs = append(form.AuthorIDs, uint(id))
	}
	in.AuthorIDs = form.AuthorIDs
	if form.Publisher != "" {
		in.Publisher = &form.Publisher
	}
	if form.Description != "" {
		in.Description = &form.Description
	}
	if form.PublishedYear != "" {
		// Нечисловий рік перетворюємо на 0 — Validate відхилить його як поза діапазоном.
		year, err := strconv.Atoi(form.PublishedYear)
		if err != nil {
			year = 0
		}
		in.PublishedYear = &year
	}
	return in, form
}

// renderForm додає до форми списки категорій та авторів і показує сторінку.
func (h *Handler) renderForm(c *gin.Context, status int, form bookForm) {
	ctx := c.Request.Context()
	categories, err := h.repos.Categories.List(ctx, 1, maxChoices)
	if err != nil {
		h.serverError(c, err)
		return
	}
	authors, err := h.repos.Authors.List(ctx, 1, maxChoices)
	if err != nil {
		h.serverError(c, err)
		return
	}
	form.Categories, form.Authors = categories, authors

	title := "Нова книга"
	if form.IsEdit {
		title = "Редагування книги"
	}
	h.view.Page(c, status, "book_form.html", h.page(c, title, form))
}

// formError показує форму з повідомленням про помилку збереження.
func (h *Handler) formError(c *gin.Context, err error, form bookForm) {
	switch {
	case errors.Is(err, repository.ErrConflict):
		form.Error = "Книга з таким ISBN уже існує"
	case errors.Is(err, repository.ErrInvalidReference):
		form.Error = "Обрана категорія або автор не існує"
	default:
		h.serverError(c, err)
		return
	}
	h.renderForm(c, http.StatusUnprocessableEntity, form)
}

// repoError показує сторінку помилки для помилки репозиторію.
func (h *Handler) repoError(c *gin.Context, err error) {
	if errors.Is(err, repository.ErrNotFound) {
		h.errorPage(c, http.StatusNotFound, "Книгу не знайдено")
		return
	}
	h.serverError(c, err)
}

// bookID читає :id з шляху. false — сторінку помилки вже показано.
func (h *Handler) bookID(c *gin.Context) (uint, bool) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil || id == 0 {
		h.errorPage(c, http.StatusBadRequest, "Некоректний id")
		return 0, false
	}
	return uint(id), true
}

// validationMessage прибирає технічний префікс "validation error: ".
func validationMessage(err error) string {
	return strings.TrimPrefix(err.Error(), model.ErrValidation.Error()+": ")
}

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
