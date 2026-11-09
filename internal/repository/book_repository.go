package repository

import (
	"context"
	"strings"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"libratrack/internal/model"
)

// BookFilter selects a page of books, optionally from one category and
// matching a search text.
type BookFilter struct {
	CategoryID *uint
	// Search — підрядок назви, ISBN або імені автора (без урахування регістру).
	// Порожній рядок означає "без фільтра".
	Search string
	Page   int
	Limit  int
}

// BookRepository is the storage contract for books. Every method takes a
// context that is passed to every database call.
type BookRepository interface {
	// Create stores the book and links it to the given authors in one
	// transaction, then fills book with its category and authors.
	// ErrInvalidReference if an author or the category does not exist,
	// ErrConflict on a duplicate ISBN.
	Create(ctx context.Context, book *model.Book, authorIDs []uint) error

	// FindByID returns the book with its category and authors, or ErrNotFound.
	FindByID(ctx context.Context, id uint) (*model.Book, error)

	// Update fully replaces the book fields and its authors. ErrNotFound if
	// the book does not exist.
	Update(ctx context.Context, id uint, book *model.Book, authorIDs []uint) (*model.Book, error)

	// Delete removes the book. The database cascades to copies and
	// book_authors. ErrNotFound if the book does not exist.
	Delete(ctx context.Context, id uint) error

	// List returns books ordered by id, each with its category and authors.
	List(ctx context.Context, filter BookFilter) ([]model.Book, error)
}

type gormBookRepository struct {
	db *gorm.DB
}

// NewBookRepository returns a GORM-backed BookRepository.
func NewBookRepository(db *gorm.DB) BookRepository {
	return &gormBookRepository{db: db}
}

// bookAuthor is a row of the book_authors join table.
type bookAuthor struct {
	AuthorID uint `gorm:"column:author_id;primaryKey"`
	BookID   uint `gorm:"column:book_id;primaryKey"`
}

func (bookAuthor) TableName() string { return "book_authors" }

func (r *gormBookRepository) Create(ctx context.Context, book *model.Book, authorIDs []uint) error {
	ids := uniqueIDs(authorIDs)

	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := requireAuthors(tx, ids); err != nil {
			return err
		}

		book.ID = 0
		if err := tx.Omit(clause.Associations).Create(book).Error; err != nil {
			return translateError(err)
		}
		if err := linkAuthors(tx, book.ID, ids); err != nil {
			return err
		}

		loaded, err := loadBook(tx, book.ID)
		if err != nil {
			return err
		}
		*book = *loaded
		return nil
	})
}

func (r *gormBookRepository) FindByID(ctx context.Context, id uint) (*model.Book, error) {
	return loadBook(r.db.WithContext(ctx), id)
}

func (r *gormBookRepository) Update(ctx context.Context, id uint, book *model.Book, authorIDs []uint) (*model.Book, error) {
	ids := uniqueIDs(authorIDs)

	var updated *model.Book
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var existing model.Book
		if err := tx.Select("id").First(&existing, id).Error; err != nil {
			return translateError(err)
		}
		if err := requireAuthors(tx, ids); err != nil {
			return err
		}

		err := tx.Model(&model.Book{}).Where("id = ?", id).
			Select("Title", "ISBN", "CategoryID", "Publisher", "PublishedYear", "Description").
			Updates(book).Error
		if err != nil {
			return translateError(err)
		}

		if err := tx.Where("book_id = ?", id).Delete(&bookAuthor{}).Error; err != nil {
			return translateError(err)
		}
		if err := linkAuthors(tx, id, ids); err != nil {
			return err
		}

		var loadErr error
		updated, loadErr = loadBook(tx, id)
		return loadErr
	})
	if err != nil {
		return nil, err
	}
	return updated, nil
}

func (r *gormBookRepository) Delete(ctx context.Context, id uint) error {
	res := r.db.WithContext(ctx).Delete(&model.Book{}, id)
	if res.Error != nil {
		return translateError(res.Error)
	}
	if res.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *gormBookRepository) List(ctx context.Context, filter BookFilter) ([]model.Book, error) {
	offset, ok := pageOffset(filter.Page, filter.Limit)
	if !ok {
		return []model.Book{}, nil
	}

	q := r.db.WithContext(ctx).Preload("Authors").Preload("Category").Model(&model.Book{}).Order("books.id ASC")
	if filter.CategoryID != nil {
		q = q.Where("books.category_id = ?", *filter.CategoryID)
	}
	if s := strings.TrimSpace(filter.Search); s != "" {
		// Значення йде лише плейсхолдером (без SQL-ін'єкцій); % і _ екрануються,
		// щоб користувач не міг підставити власний шаблон LIKE.
		pattern := "%" + escapeLike(s) + "%"
		q = q.Where(`(books.title ILIKE ? OR books.isbn ILIKE ? OR EXISTS (
			SELECT 1 FROM book_authors ba JOIN authors a ON a.id = ba.author_id
			WHERE ba.book_id = books.id AND a.full_name ILIKE ?))`, pattern, pattern, pattern)
	}

	books := []model.Book{}
	if err := q.Limit(filter.Limit).Offset(offset).Find(&books).Error; err != nil {
		return nil, translateError(err)
	}

	return books, nil
}

// loadBook reads one book together with its category and authors.
func loadBook(db *gorm.DB, id uint) (*model.Book, error) {
	var book model.Book
	err := db.Preload("Category").Preload("Authors").First(&book, id).Error
	if err != nil {
		return nil, translateError(err)
	}
	return &book, nil
}

// requireAuthors returns ErrInvalidReference unless every id is an existing author.
func requireAuthors(tx *gorm.DB, ids []uint) error {
	var n int64
	if err := tx.Model(&model.Author{}).Where("id IN ?", ids).Count(&n).Error; err != nil {
		return translateError(err)
	}
	if int(n) != len(ids) {
		return ErrInvalidReference
	}
	return nil
}

// linkAuthors inserts the book_authors rows of one book.
func linkAuthors(tx *gorm.DB, bookID uint, authorIDs []uint) error {
	rows := make([]bookAuthor, 0, len(authorIDs))
	for _, id := range authorIDs {
		rows = append(rows, bookAuthor{AuthorID: id, BookID: bookID})
	}
	return translateError(tx.Create(&rows).Error)
}

// likeEscaper екранує спецсимволи LIKE (\ — стандартний escape у PostgreSQL).
var likeEscaper = strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`)

func escapeLike(s string) string { return likeEscaper.Replace(s) }

// uniqueIDs removes duplicates and keeps the order of first appearance.
func uniqueIDs(ids []uint) []uint {
	seen := make(map[uint]struct{}, len(ids))
	out := make([]uint, 0, len(ids))
	for _, id := range ids {
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		out = append(out, id)
	}
	return out
}
