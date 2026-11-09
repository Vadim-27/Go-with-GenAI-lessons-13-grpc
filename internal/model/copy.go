package model

// Allowed values of Copy.Status. They mirror the CHECK constraint in the
// database, so an invalid value is rejected by both layers.
const (
	StatusAvailable   = "available"
	StatusBorrowed    = "borrowed"
	StatusLost        = "lost"
	StatusUnderRepair = "under_repair"
)

// ValidCopyStatus reports whether s is one of the allowed copy statuses.
func ValidCopyStatus(s string) bool {
	switch s {
	case StatusAvailable, StatusBorrowed, StatusLost, StatusUnderRepair:
		return true
	}
	return false
}

// Copy is one physical copy of a book.
type Copy struct {
	ID          uint   `gorm:"primaryKey" json:"id"`
	BookID      uint   `gorm:"column:book_id;not null" json:"book_id"`
	InventoryNo string `gorm:"column:inventory_no;not null" json:"inventory_no"`
	Status      string `gorm:"not null;default:available" json:"status"`
}

// TableName pins the table name ("copies", not "copys").
func (Copy) TableName() string { return "copies" }

// Availability summarises the copies of one book.
type Availability struct {
	BookID          uint  `json:"book_id"`
	TotalCopies     int64 `json:"total_copies"`
	AvailableCopies int64 `json:"available_copies"`
}
