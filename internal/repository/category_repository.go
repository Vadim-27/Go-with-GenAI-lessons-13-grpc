package repository

import (
	"context"

	"gorm.io/gorm"

	"libratrack/internal/model"
)

// CategoryRepository is the storage contract for categories.
type CategoryRepository interface {
	// Create stores the category and fills its ID. ErrConflict on a duplicate name.
	Create(ctx context.Context, category *model.Category) error
	// FindByID returns the category or ErrNotFound.
	FindByID(ctx context.Context, id uint) (*model.Category, error)
	// List returns categories ordered by id. page and limit must be >= 1.
	List(ctx context.Context, page, limit int) ([]model.Category, error)
}

type gormCategoryRepository struct {
	db *gorm.DB
}

// NewCategoryRepository returns a GORM-backed CategoryRepository.
func NewCategoryRepository(db *gorm.DB) CategoryRepository {
	return &gormCategoryRepository{db: db}
}

func (r *gormCategoryRepository) Create(ctx context.Context, category *model.Category) error {
	return translateError(r.db.WithContext(ctx).Create(category).Error)
}

func (r *gormCategoryRepository) FindByID(ctx context.Context, id uint) (*model.Category, error) {
	var category model.Category
	if err := r.db.WithContext(ctx).First(&category, id).Error; err != nil {
		return nil, translateError(err)
	}
	return &category, nil
}

func (r *gormCategoryRepository) List(ctx context.Context, page, limit int) ([]model.Category, error) {
	offset, ok := pageOffset(page, limit)
	if !ok {
		return []model.Category{}, nil
	}

	categories := []model.Category{}
	err := r.db.WithContext(ctx).Order("id ASC").Limit(limit).Offset(offset).Find(&categories).Error
	return categories, translateError(err)
}
