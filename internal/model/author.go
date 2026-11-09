package model

// Author is a person who wrote one or more books. Bio is optional, so it is
// a pointer: "not set" is stored as NULL and serialised as JSON null.
type Author struct {
	ID       uint    `gorm:"primaryKey" json:"id"`
	FullName string  `gorm:"column:full_name;not null" json:"full_name"`
	Bio      *string `json:"bio"`
}

// TableName pins the table name.
func (Author) TableName() string { return "authors" }
