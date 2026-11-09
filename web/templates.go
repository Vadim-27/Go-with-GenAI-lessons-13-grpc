// Package web містить HTML-шаблони та статичні файли веб-інтерфейсу.
//
// Обидва набори вбудовуються в бінарник директивою go:embed, тож програмі не
// потрібні папки web/templates та web/static на диску під час виконання.
package web

import "embed"

// TemplatesFS — дерево templates/ (layout, сторінки, partials).
// Шаблони розбираються з нього через template.ParseFS, а не ParseGlob з диска.
//
//go:embed templates
var TemplatesFS embed.FS
