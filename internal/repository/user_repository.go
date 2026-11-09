package repository

import (
	"context"

	"gorm.io/gorm"

	"libratrack/internal/model"
)

// UserRepository — контракт сховища користувачів.
type UserRepository interface {
	// Create зберігає користувача і заповнює його ID.
	// Повертає ErrConflict, якщо email уже зайнятий.
	Create(ctx context.Context, user *model.User) error
	// FindByEmail повертає користувача або ErrNotFound.
	FindByEmail(ctx context.Context, email string) (*model.User, error)
	// FindByID повертає користувача або ErrNotFound.
	FindByID(ctx context.Context, id uint) (*model.User, error)
	// List повертає користувачів, впорядкованих за id. page і limit >= 1.
	List(ctx context.Context, page, limit int) ([]model.User, error)
	// UpdateRole змінює роль і повертає оновленого користувача або ErrNotFound.
	UpdateRole(ctx context.Context, id uint, role string) (*model.User, error)
	// UpdateFullName змінює ім'я (nil = очистити) і повертає користувача або ErrNotFound.
	UpdateFullName(ctx context.Context, id uint, fullName *string) (*model.User, error)
}

type gormUserRepository struct {
	db *gorm.DB
}

// NewUserRepository повертає UserRepository на основі GORM.
func NewUserRepository(db *gorm.DB) UserRepository {
	return &gormUserRepository{db: db}
}

func (r *gormUserRepository) Create(ctx context.Context, user *model.User) error {
	return translateError(r.db.WithContext(ctx).Create(user).Error)
}

func (r *gormUserRepository) FindByEmail(ctx context.Context, email string) (*model.User, error) {
	var user model.User
	// Параметризований запит (плейсхолдер ?) — без SQL-ін'єкцій.
	if err := r.db.WithContext(ctx).Where("email = ?", email).First(&user).Error; err != nil {
		return nil, translateError(err)
	}
	return &user, nil
}

func (r *gormUserRepository) FindByID(ctx context.Context, id uint) (*model.User, error) {
	var user model.User
	if err := r.db.WithContext(ctx).First(&user, id).Error; err != nil {
		return nil, translateError(err)
	}
	return &user, nil
}

func (r *gormUserRepository) List(ctx context.Context, page, limit int) ([]model.User, error) {
	offset, ok := pageOffset(page, limit)
	if !ok {
		return []model.User{}, nil
	}
	users := []model.User{}
	err := r.db.WithContext(ctx).Order("id ASC").Limit(limit).Offset(offset).Find(&users).Error
	return users, translateError(err)
}

func (r *gormUserRepository) UpdateRole(ctx context.Context, id uint, role string) (*model.User, error) {
	return r.updateColumn(ctx, id, "role", role)
}

func (r *gormUserRepository) UpdateFullName(ctx context.Context, id uint, fullName *string) (*model.User, error) {
	return r.updateColumn(ctx, id, "full_name", fullName)
}

// updateColumn змінює одну колонку (назва — константа з коду, не ввід користувача)
// і повертає оновлений рядок.
func (r *gormUserRepository) updateColumn(ctx context.Context, id uint, column string, value any) (*model.User, error) {
	res := r.db.WithContext(ctx).Model(&model.User{}).Where("id = ?", id).Update(column, value)
	if res.Error != nil {
		return nil, translateError(res.Error)
	}
	if res.RowsAffected == 0 {
		return nil, ErrNotFound
	}
	return r.FindByID(ctx, id)
}
