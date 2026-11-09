package repository

import (
	"errors"
	"fmt"
	"io/fs"
	"strings"

	"github.com/golang-migrate/migrate/v4"
	// Database driver (pgx v5) for golang-migrate.
	_ "github.com/golang-migrate/migrate/v4/database/pgx/v5"
	"github.com/golang-migrate/migrate/v4/source/iofs"
)

// Migrate applies all pending SQL migrations from src to the database. The
// schema is created only by these files, never by the ORM.
//
// src is usually the embedded migrations.FS, so the binary does not need the
// migrations folder on disk; os.DirFS(dir) works as well.
func Migrate(dsn string, src fs.FS) error {
	driver, err := iofs.New(src, ".")
	if err != nil {
		return fmt.Errorf("open migrations: %w", err)
	}
	m, err := migrate.NewWithSourceInstance("iofs", driver, pgxURL(dsn))
	if err != nil {
		return fmt.Errorf("init migrations: %w", err)
	}
	defer m.Close()

	if err := m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return fmt.Errorf("apply migrations: %w", err)
	}
	return nil
}

// pgxURL rewrites a postgres:// URL to the scheme registered by the pgx v5
// driver of golang-migrate.
func pgxURL(dsn string) string {
	for _, prefix := range []string{"postgresql://", "postgres://"} {
		if strings.HasPrefix(dsn, prefix) {
			return "pgx5://" + strings.TrimPrefix(dsn, prefix)
		}
	}
	return dsn
}
