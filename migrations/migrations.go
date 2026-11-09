// Package migrations вбудовує SQL-міграції у бінарник.
//
// Без цього сервер шукав би папку ./migrations поруч із собою і не стартував
// би на іншій машині чи в контейнері FROM scratch.
package migrations

import "embed"

// FS містить усі файли *.sql цієї папки (вбудовуються під час компіляції).
//
//go:embed *.sql
var FS embed.FS
