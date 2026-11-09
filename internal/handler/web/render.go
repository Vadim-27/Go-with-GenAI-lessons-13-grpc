// Package web — HTML-шар LibraTrack: сторінки на html/template та HTMX-фрагменти.
//
// Хендлери працюють з тими самими інтерфейсами репозиторіїв, що й JSON API,
// а шаблони беруться з файлової системи fs.FS (у продакшні — вбудований
// embed.FS з пакета libratrack/web), тому бінарнику не потрібні файли на диску.
package web

import (
	"bytes"
	"fmt"
	"html/template"
	"io/fs"
	"log"
	"net/http"
	"slices"

	"github.com/gin-gonic/gin"
)

// pages — сторінки, кожна з яких перевизначає блок "content" у layout.html.
var pages = []string{"catalog.html", "book_form.html", "login.html", "register.html", "error.html"}

// funcs — допоміжні функції шаблонів. Жодна не повертає template.HTML:
// усі дані з БД та від користувача проходять контекстне автоекранування.
var funcs = template.FuncMap{
	// hasID повідомляє, чи є id серед ids (позначка selected у списку авторів).
	"hasID": func(ids []uint, id uint) bool { return slices.Contains(ids, id) },
}

// Renderer тримає окремий набір шаблонів на кожну сторінку: layout + partials +
// сторінка. Окремі набори потрібні, бо всі сторінки визначають блок з однаковою
// назвою "content", і в одному наборі останнє визначення перекрило б решту.
type Renderer struct {
	pages map[string]*template.Template
}

// NewRenderer розбирає шаблони з fsys через template.ParseFS (не ParseGlob з
// диска). fsys має містити каталог templates/. Помилка в шаблоні виявляється
// одразу під час старту, а не на першому запиті.
func NewRenderer(fsys fs.FS) (*Renderer, error) {
	r := &Renderer{pages: make(map[string]*template.Template, len(pages))}
	for _, page := range pages {
		t, err := template.New("layout.html").Funcs(funcs).ParseFS(fsys,
			"templates/layout.html",
			"templates/partials/*.html",
			"templates/"+page,
		)
		if err != nil {
			return nil, fmt.Errorf("parse template %s: %w", page, err)
		}
		r.pages[page] = t
	}
	return r, nil
}

// Page рендерить повну сторінку (layout.html з блоком "content" сторінки page).
func (r *Renderer) Page(c *gin.Context, status int, page string, data any) {
	r.execute(c, status, page, "layout.html", data)
}

// Fragment рендерить лише іменований шаблон (partial) без layout — відповідь на
// HTMX-запит, яку бібліотека вставляє у вже відкриту сторінку.
func (r *Renderer) Fragment(c *gin.Context, status int, name string, data any) {
	r.execute(c, status, "catalog.html", name, data)
}

// execute спочатку рендерить у буфер: якщо шаблон впаде посередині, клієнт
// отримає 500, а не обрізаний HTML зі статусом 200.
func (r *Renderer) execute(c *gin.Context, status int, page, name string, data any) {
	t, ok := r.pages[page]
	if !ok {
		log.Printf("render: unknown page %q", page)
		c.String(http.StatusInternalServerError, "Internal server error")
		return
	}
	var buf bytes.Buffer
	if err := t.ExecuteTemplate(&buf, name, data); err != nil {
		log.Printf("render %s/%s: %v", page, name, err)
		c.String(http.StatusInternalServerError, "Internal server error")
		return
	}
	c.Data(status, "text/html; charset=utf-8", buf.Bytes())
}
