package model

import "time"

// Loan — позика одного примірника читачу.
// ReturnedAt == nil означає, що книгу ще не повернуто.
type Loan struct {
	ID         uint       `gorm:"primaryKey" json:"id"`
	UserID     uint       `gorm:"column:user_id;not null" json:"user_id"`
	CopyID     uint       `gorm:"column:copy_id;not null" json:"copy_id"`
	IssuedBy   uint       `gorm:"column:issued_by;not null" json:"issued_by"`
	IssuedAt   time.Time  `gorm:"column:issued_at;not null" json:"issued_at"`
	DueAt      time.Time  `gorm:"column:due_at;not null" json:"due_at"`
	ReturnedAt *time.Time `gorm:"column:returned_at" json:"returned_at"`
}

// TableName фіксує назву таблиці.
func (Loan) TableName() string { return "loans" }
