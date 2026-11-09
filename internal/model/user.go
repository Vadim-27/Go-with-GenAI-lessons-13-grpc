package model

import "time"

// Ролі користувачів. Збігаються з CHECK ck_users_role у базі даних.
const (
	RoleReader    = "reader"
	RoleLibrarian = "librarian"
	RoleAdmin     = "admin"
)

// User is a library account. The password hash is never serialised.
type User struct {
	ID           uint      `gorm:"primaryKey" json:"id"`
	Email        string    `gorm:"not null" json:"email"`
	PasswordHash string    `gorm:"column:password_hash;not null" json:"-"`
	FullName     *string   `gorm:"column:full_name" json:"full_name"`
	Role         string    `gorm:"not null;default:reader" json:"role"`
	CreatedAt    time.Time `json:"created_at"`
}

// TableName pins the table name.
func (User) TableName() string { return "users" }
