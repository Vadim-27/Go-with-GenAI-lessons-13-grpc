// Package testutil prepares a real PostgreSQL database for tests.
package testutil

import (
	"fmt"
	"os"
	"strings"
	"testing"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"libratrack/internal/repository"
	"libratrack/migrations"
)

// NewDB returns a connection to a clean, fully migrated database schema.
//
// Go runs the test packages in parallel, so every package passes its own
// schema name: packages never see each other's rows. The schema is dropped
// and recreated, the real (embedded) SQL migrations are applied to it, and the demo seed
// rows are removed so each test starts from empty tables.
//
// The test is skipped when TEST_DATABASE_URL is not set.
func NewDB(t *testing.T, schema string) *gorm.DB {
	t.Helper()

	base := os.Getenv("TEST_DATABASE_URL")
	if base == "" {
		t.Skip("TEST_DATABASE_URL is not set")
	}

	quiet := &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)}

	admin, err := gorm.Open(postgres.Open(base), quiet)
	if err != nil {
		t.Fatalf("connect to the test database: %v", err)
	}
	reset := fmt.Sprintf("DROP SCHEMA IF EXISTS %s CASCADE; CREATE SCHEMA %s", schema, schema)
	if err := admin.Exec(reset).Error; err != nil {
		t.Fatalf("recreate schema %s: %v", schema, err)
	}

	dsn := withSearchPath(base, schema)
	if err := repository.Migrate(dsn, migrations.FS); err != nil {
		t.Fatalf("apply migrations: %v", err)
	}

	db, err := gorm.Open(postgres.Open(dsn), quiet)
	if err != nil {
		t.Fatalf("connect to schema %s: %v", schema, err)
	}
	clean := "TRUNCATE loans, book_authors, copies, books, authors, categories, users RESTART IDENTITY CASCADE"
	if err := db.Exec(clean).Error; err != nil {
		t.Fatalf("remove seed data: %v", err)
	}

	t.Cleanup(func() {
		for _, g := range []*gorm.DB{db, admin} {
			if sqlDB, err := g.DB(); err == nil {
				_ = sqlDB.Close()
			}
		}
	})
	return db
}

// withSearchPath appends the search_path parameter to a PostgreSQL URL.
func withSearchPath(dsn, schema string) string {
	sep := "?"
	if strings.Contains(dsn, "?") {
		sep = "&"
	}
	return dsn + sep + "search_path=" + schema
}
