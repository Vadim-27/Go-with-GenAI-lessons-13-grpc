package repository

import (
	"context"

	"gorm.io/gorm"

	"libratrack/internal/model"
)

// AuthorRepository is the storage contract for authors.
type AuthorRepository interface {
	// Create stores the author and fills its ID.
	Create(ctx context.Context, author *model.Author) error
	// FindByID returns the author or ErrNotFound.
	FindByID(ctx context.Context, id uint) (*model.Author, error)
	// List returns authors ordered by id. page and limit must be >= 1.
	List(ctx context.Context, page, limit int) ([]model.Author, error)
}

type gormAuthorRepository struct {
	db *gorm.DB
}

// NewAuthorRepository returns a GORM-backed AuthorRepository.
func NewAuthorRepository(db *gorm.DB) AuthorRepository {
	return &gormAuthorRepository{db: db}
}

func (r *gormAuthorRepository) Create(ctx context.Context, author *model.Author) error {
	return translateError(r.db.WithContext(ctx).Create(author).Error)
}

func (r *gormAuthorRepository) FindByID(ctx context.Context, id uint) (*model.Author, error) {
	var author model.Author
	if err := r.db.WithContext(ctx).First(&author, id).Error; err != nil {
		return nil, translateError(err)
	}
	return &author, nil
}

func (r *gormAuthorRepository) List(ctx context.Context, page, limit int) ([]model.Author, error) {
	offset, ok := pageOffset(page, limit)
	if !ok {
		return []model.Author{}, nil
	}

	authors := []model.Author{}
	err := r.db.WithContext(ctx).Order("id ASC").Limit(limit).Offset(offset).Find(&authors).Error
	return authors, translateError(err)
}
