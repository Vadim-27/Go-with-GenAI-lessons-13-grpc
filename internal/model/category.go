// Package model holds the domain types, request inputs and error codes.
package model

// Category groups books (one category has many books).
type Category struct {
	ID   uint   `gorm:"primaryKey" json:"id"`
	Name string `gorm:"not null" json:"name"`
}

// TableName pins the table name so GORM never guesses a plural form.
func (Category) TableName() string { return "categories" }
