package web

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/gin-gonic/gin"

	"libratrack/internal/middleware"
	"libratrack/internal/repository"
)

const (
	// catalogLimit — скільки книг показує одна сторінка каталогу.
	catalogLimit = 100
	// maxQueryLen — максимальна довжина пошукового запиту (у символах).
	maxQueryLen = 100
)

// bookRow — дані одного рядка таблиці (partials/book_row.html).
type bookRow struct {
	ID        uint
	Title     string
	ISBN      string
	Category  string
	Authors   string
	Year      *int
	Total     int64
	Available int64
	CanManage bool
}

// bookRows — дані partial "book_rows" (тіло таблиці).
type bookRows struct {
	Rows      []bookRow
	CanManage bool
}

type catalogData struct {
	Query string
	List  bookRows
}

// Catalog відповідає на GET /catalog: повна сторінка з таблицею і пошуком.
func (h *Handler) Catalog(c *gin.Context) {
	q := searchQuery(c)
	list, err := h.loadRows(c.Request.Context(), q, canManage(c.GetString(middleware.CtxRole)))
	if err != nil {
		h.serverError(c, err)
		return
	}
	h.view.Page(c, http.StatusOK, "catalog.html", h.page(c, "Каталог", catalogData{Query: q, List: list}))
}

// BookList відповідає на GET /web/books?q= (HTMX live search).
//
// Для HTMX-запиту (HX-Request: true) повертає ЛИШЕ рядки <tr> — HTMX вставляє
// їх у #book-list. Звичайний запит браузера переадресовується на повну сторінку.
func (h *Handler) BookList(c *gin.Context) {
	q := searchQuery(c)
	// Відповідь залежить від заголовка HX-Request: кеш не повинен їх змішувати.
	c.Header("Vary", "HX-Request")
	if !middleware.IsHTMX(c) {
		c.Redirect(http.StatusSeeOther, "/catalog?q="+url.QueryEscape(q))
		return
	}

	list, err := h.loadRows(c.Request.Context(), q, canManage(c.GetString(middleware.CtxRole)))
	if err != nil {
		h.serverError(c, err)
		return
	}
	h.view.Fragment(c, http.StatusOK, "book_rows", list)
}

// DeleteBook відповідає на DELETE /web/books/:id (лише librarian/admin).
//
// Успіх — 200 OK з ПОРОЖНІМ тілом: HTMX замінює (hx-swap="outerHTML") цільовий
// рядок (hx-target="closest tr") порожнечею, тобто рядок зникає без перезавантаження.
// 204 не підходить: на 204 HTMX нічого не замінює.
func (h *Handler) DeleteBook(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil || id == 0 {
		c.String(http.StatusBadRequest, "Некоректний id")
		return
	}

	err = h.repos.Books.Delete(c.Request.Context(), uint(id))
	switch {
	case err == nil:
		c.Status(http.StatusOK)
	case errors.Is(err, repository.ErrNotFound):
		c.String(http.StatusNotFound, "Книгу не знайдено")
	case errors.Is(err, repository.ErrInvalidReference):
		// Примірники книги мають історію позик (FK ON DELETE RESTRICT).
		c.String(http.StatusConflict, "Книгу не можна видалити: її примірники мають історію позик")
	default:
		h.serverError(c, err)
	}
}

// loadRows читає книги та наявність примірників двома запитами (без N+1).
func (h *Handler) loadRows(ctx context.Context, q string, manage bool) (bookRows, error) {
	books, err := h.repos.Books.List(ctx, repository.BookFilter{Page: 1, Limit: catalogLimit, Search: q})
	if err != nil {
		return bookRows{}, err
	}

	ids := make([]uint, len(books))
	for i, b := range books {
		ids[i] = b.ID
	}
	avail, err := h.repos.Copies.AvailabilityByBooks(ctx, ids)
	if err != nil {
		return bookRows{}, err
	}

	rows := make([]bookRow, len(books))
	for i, b := range books {
		names := make([]string, len(b.Authors))
		for j, a := range b.Authors {
			names[j] = a.FullName
		}
		rows[i] = bookRow{
			ID:        b.ID,
			Title:     b.Title,
			ISBN:      b.ISBN,
			Category:  b.Category.Name,
			Authors:   strings.Join(names, ", "),
			Year:      b.PublishedYear,
			Total:     avail[b.ID].TotalCopies,
			Available: avail[b.ID].AvailableCopies,
			CanManage: manage,
		}
	}
	return bookRows{Rows: rows, CanManage: manage}, nil
}

// searchQuery повертає параметр q без пробілів по краях, обрізаний до maxQueryLen символів.
func searchQuery(c *gin.Context) string {
	q := strings.TrimSpace(c.Query("q"))
	if utf8.RuneCountInString(q) > maxQueryLen {
		q = string([]rune(q)[:maxQueryLen])
	}
	return q
}
