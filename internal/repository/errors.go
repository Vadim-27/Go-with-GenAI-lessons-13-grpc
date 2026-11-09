// Package repository contains the GORM-backed storage layer. Handlers depend
// on the interfaces declared here, not on GORM.
package repository

import (
	"errors"

	"github.com/jackc/pgx/v5/pgconn"
	"gorm.io/gorm"
)

// ErrNotFound is returned when the requested row does not exist.
var ErrNotFound = errors.New("not found")

// ErrConflict is returned when a UNIQUE constraint is violated (duplicate ISBN, name, inventory number).
var ErrConflict = errors.New("conflict")

// ErrInvalidReference is returned when a foreign key points to a row that does not exist.
var ErrInvalidReference = errors.New("invalid reference")

// PostgreSQL error codes that are mapped to the sentinel errors above.
const (
	pgUniqueViolation     = "23505"
	pgForeignKeyViolation = "23503"
)

// translateError converts GORM and PostgreSQL errors into the sentinel
// errors of this package. Unknown errors are returned unchanged.
func translateError(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return ErrNotFound
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		switch pgErr.Code {
		case pgUniqueViolation:
			return ErrConflict
		case pgForeignKeyViolation:
			return ErrInvalidReference
		}
	}
	return err
}

// maxOffset keeps (page-1)*limit far away from integer overflow.
const maxOffset = 1 << 31

// pageOffset converts page and limit into an SQL offset. The second result
// is false when the page is so far out of range that the answer is empty.
func pageOffset(page, limit int) (int, bool) {
	if page < 1 || limit < 1 || page-1 > maxOffset/limit {
		return 0, false
	}
	return (page - 1) * limit, true
}

// ErrCopyUnavailable повертається, коли примірник не можна видати
// (він уже виданий, загублений або на ремонті).
var ErrCopyUnavailable = errors.New("copy is not available")

// ErrAlreadyReturned повертається при повторному поверненні тієї самої позики.
var ErrAlreadyReturned = errors.New("loan already returned")
