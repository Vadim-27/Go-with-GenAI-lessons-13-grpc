package repository

import (
	"context"

	"gorm.io/gorm"

	"libratrack/internal/model"
)

// CopyRepository is the storage contract for physical copies of books.
type CopyRepository interface {
	// Create stores a copy of an existing book. ErrNotFound if the book does
	// not exist, ErrConflict on a duplicate inventory number.
	Create(ctx context.Context, copy *model.Copy) error
	// ListByBook returns the copies of a book ordered by id, or ErrNotFound
	// if the book does not exist.
	ListByBook(ctx context.Context, bookID uint) ([]model.Copy, error)
	// Availability counts all and available copies of a book, or ErrNotFound
	// if the book does not exist.
	Availability(ctx context.Context, bookID uint) (model.Availability, error)
	// AvailabilityByBooks рахує примірники кількох книг ОДНИМ запитом (GROUP BY),
	// щоб сторінка каталогу не робила N запитів. Книги без примірників у мапі відсутні.
	AvailabilityByBooks(ctx context.Context, bookIDs []uint) (map[uint]model.Availability, error)
}

type gormCopyRepository struct {
	db *gorm.DB
}

// NewCopyRepository returns a GORM-backed CopyRepository.
func NewCopyRepository(db *gorm.DB) CopyRepository {
	return &gormCopyRepository{db: db}
}

func (r *gormCopyRepository) Create(ctx context.Context, copy *model.Copy) error {
	if err := r.requireBook(ctx, copy.BookID); err != nil {
		return err
	}
	return translateError(r.db.WithContext(ctx).Create(copy).Error)
}

func (r *gormCopyRepository) ListByBook(ctx context.Context, bookID uint) ([]model.Copy, error) {
	if err := r.requireBook(ctx, bookID); err != nil {
		return nil, err
	}

	copies := []model.Copy{}
	err := r.db.WithContext(ctx).Where("book_id = ?", bookID).Order("id ASC").Find(&copies).Error
	return copies, translateError(err)
}

func (r *gormCopyRepository) Availability(ctx context.Context, bookID uint) (model.Availability, error) {
	if err := r.requireBook(ctx, bookID); err != nil {
		return model.Availability{}, err
	}

	var row struct {
		Total     int64
		Available int64
	}
	err := r.db.WithContext(ctx).Model(&model.Copy{}).
		Select("COUNT(*) AS total, COUNT(*) FILTER (WHERE status = ?) AS available", model.StatusAvailable).
		Where("book_id = ?", bookID).
		Scan(&row).Error
	if err != nil {
		return model.Availability{}, translateError(err)
	}
	return model.Availability{BookID: bookID, TotalCopies: row.Total, AvailableCopies: row.Available}, nil
}

// requireBook returns ErrNotFound when the book does not exist.
func (r *gormCopyRepository) requireBook(ctx context.Context, bookID uint) error {
	var n int64
	if err := r.db.WithContext(ctx).Model(&model.Book{}).Where("id = ?", bookID).Count(&n).Error; err != nil {
		return translateError(err)
	}
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *gormCopyRepository) AvailabilityByBooks(ctx context.Context, bookIDs []uint) (map[uint]model.Availability, error) {
	result := make(map[uint]model.Availability, len(bookIDs))
	if len(bookIDs) == 0 {
		return result, nil
	}

	var rows []struct {
		BookID    uint
		Total     int64
		Available int64
	}
	err := r.db.WithContext(ctx).Model(&model.Copy{}).
		Select("book_id, COUNT(*) AS total, COUNT(*) FILTER (WHERE status = ?) AS available", model.StatusAvailable).
		Where("book_id IN ?", bookIDs).
		Group("book_id").
		Scan(&rows).Error
	if err != nil {
		return nil, translateError(err)
	}
	for _, row := range rows {
		result[row.BookID] = model.Availability{BookID: row.BookID, TotalCopies: row.Total, AvailableCopies: row.Available}
	}
	return result, nil
}
