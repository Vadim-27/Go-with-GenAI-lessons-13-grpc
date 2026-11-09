package repository

import (
	"context"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"libratrack/internal/model"
)

// LoanRepository — контракт сховища позик.
type LoanRepository interface {
	// Issue видає примірник читачу в одній транзакції: створює позику і ставить
	// примірнику статус borrowed. ErrNotFound — примірника немає,
	// ErrCopyUnavailable — він не в наявності, ErrInvalidReference — немає користувача.
	Issue(ctx context.Context, userID, copyID, issuedBy uint, days int) (*model.Loan, error)
	// Return закриває позику і повертає примірник у наявність (транзакція).
	// ErrNotFound — позики немає, ErrAlreadyReturned — вже повернено.
	Return(ctx context.Context, loanID uint) (*model.Loan, error)
	// ListByUser повертає позики читача, найновіші першими.
	ListByUser(ctx context.Context, userID uint, page, limit int) ([]model.Loan, error)
}

type gormLoanRepository struct {
	db *gorm.DB
}

// NewLoanRepository повертає LoanRepository на основі GORM.
func NewLoanRepository(db *gorm.DB) LoanRepository {
	return &gormLoanRepository{db: db}
}

func (r *gormLoanRepository) Issue(ctx context.Context, userID, copyID, issuedBy uint, days int) (*model.Loan, error) {
	var loan model.Loan
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// SELECT ... FOR UPDATE: блокуємо рядок примірника, щоб двоє працівників
		// одночасно не видали той самий примірник (гонка).
		var cp model.Copy
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&cp, copyID).Error; err != nil {
			return err
		}
		if cp.Status != model.StatusAvailable {
			return ErrCopyUnavailable
		}

		now := time.Now()
		loan = model.Loan{
			UserID:   userID,
			CopyID:   copyID,
			IssuedBy: issuedBy,
			IssuedAt: now,
			DueAt:    now.AddDate(0, 0, days),
		}
		if err := tx.Create(&loan).Error; err != nil {
			return err
		}
		return tx.Model(&model.Copy{}).Where("id = ?", copyID).
			Update("status", model.StatusBorrowed).Error
	})
	if err != nil {
		return nil, translateError(err)
	}
	return &loan, nil
}

func (r *gormLoanRepository) Return(ctx context.Context, loanID uint) (*model.Loan, error) {
	var loan model.Loan
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&loan, loanID).Error; err != nil {
			return err
		}
		if loan.ReturnedAt != nil {
			return ErrAlreadyReturned
		}

		now := time.Now()
		if err := tx.Model(&loan).Update("returned_at", now).Error; err != nil {
			return err
		}
		loan.ReturnedAt = &now

		// Повертаємо в наявність лише якщо примірник досі borrowed
		// (щоб не затерти статус lost/under_repair, виставлений вручну).
		return tx.Model(&model.Copy{}).
			Where("id = ? AND status = ?", loan.CopyID, model.StatusBorrowed).
			Update("status", model.StatusAvailable).Error
	})
	if err != nil {
		return nil, translateError(err)
	}
	return &loan, nil
}

func (r *gormLoanRepository) ListByUser(ctx context.Context, userID uint, page, limit int) ([]model.Loan, error) {
	offset, ok := pageOffset(page, limit)
	if !ok {
		return []model.Loan{}, nil
	}
	loans := []model.Loan{}
	// Фільтр user_id — єдине, що відділяє «мої» позики від чужих.
	err := r.db.WithContext(ctx).Where("user_id = ?", userID).
		Order("id DESC").Limit(limit).Offset(offset).Find(&loans).Error
	return loans, translateError(err)
}
