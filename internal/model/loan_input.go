package model

// Строк позики у днях: значення за замовчуванням та верхня межа.
const (
	DefaultLoanDays = 14
	maxLoanDays     = 60
)

// LoanInput — тіло POST /loans.
// issued_by беремо з токена, а не з тіла: працівник не може видати книгу «від імені» іншого.
type LoanInput struct {
	UserID *uint `json:"user_id"`
	CopyID *uint `json:"copy_id"`
	Days   *int  `json:"days"`
}

// Validate перевіряє обов'язкові id та межі строку.
func (in LoanInput) Validate() error {
	if in.UserID == nil || *in.UserID == 0 {
		return invalid("user_id is required")
	}
	if in.CopyID == nil || *in.CopyID == 0 {
		return invalid("copy_id is required")
	}
	if in.Days != nil && (*in.Days < 1 || *in.Days > maxLoanDays) {
		return invalid("days must be between 1 and %d", maxLoanDays)
	}
	return nil
}

// DaysOrDefault повертає строк позики у днях.
func (in LoanInput) DaysOrDefault() int {
	if in.Days == nil {
		return DefaultLoanDays
	}
	return *in.Days
}
