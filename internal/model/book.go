package model

import "time"

// Book is a stored book. The category is a One-to-Many relation and the
// authors are Many-to-Many through the book_authors join table. Optional
// columns are pointers so that "not set" stays NULL instead of a zero value.
type Book struct {
	ID            uint      `gorm:"primaryKey" json:"id"`
	Title         string    `gorm:"not null" json:"title"`
	ISBN          string    `gorm:"column:isbn;not null" json:"isbn"`
	CategoryID    uint      `gorm:"not null" json:"category_id"`
	Category      Category  `gorm:"foreignKey:CategoryID" json:"category"`
	Authors       []Author  `gorm:"many2many:book_authors;" json:"authors"`
	Publisher     *string   `json:"publisher"`
	PublishedYear *int      `json:"published_year"`
	Description   *string   `json:"description"`
	CreatedAt     time.Time `json:"created_at"`
}

// TableName pins the table name.
func (Book) TableName() string { return "books" }
