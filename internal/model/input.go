package model

import (
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"
)

// ErrValidation marks every validation failure. Callers detect it with
// errors.Is(err, ErrValidation) and answer 422 validation_error.
var ErrValidation = errors.New("validation error")

// Year bounds mirror the CHECK constraint on books.published_year.
const (
	minPublishedYear = 1000
	maxPublishedYear = 2100
)

func blank(s *string) bool {
	return s == nil || strings.TrimSpace(*s) == ""
}

func invalid(format string, args ...any) error {
	return fmt.Errorf("%w: %s", ErrValidation, fmt.Sprintf(format, args...))
}

// CategoryInput is the request body for POST /categories.
type CategoryInput struct {
	Name *string `json:"name"`
}

// Name length bounds (in characters) mirror ck_categories_name_len in the database.
const (
	minCategoryName = 2
	maxCategoryName = 40
)

// Validate requires a name of 2 to 40 characters after trimming spaces.
func (in CategoryInput) Validate() error {
	if blank(in.Name) {
		return invalid("name is required")
	}
	// RuneCount counts characters, not bytes: the database uses char_length too.
	n := utf8.RuneCountInString(strings.TrimSpace(*in.Name))
	if n < minCategoryName || n > maxCategoryName {
		return invalid("name must be %d to %d characters long", minCategoryName, maxCategoryName)
	}
	return nil
}

// ToCategory converts a validated input into a Category.
func (in CategoryInput) ToCategory() Category {
	return Category{Name: strings.TrimSpace(*in.Name)}
}

// AuthorInput is the request body for POST /authors.
type AuthorInput struct {
	FullName *string `json:"full_name"`
	Bio      *string `json:"bio"`
}

// Validate requires a non-blank full_name.
func (in AuthorInput) Validate() error {
	if blank(in.FullName) {
		return invalid("full_name is required")
	}
	return nil
}

// ToAuthor converts a validated input into an Author.
func (in AuthorInput) ToAuthor() Author {
	return Author{FullName: strings.TrimSpace(*in.FullName), Bio: in.Bio}
}

// BookInput is the request body for POST /books and PUT /books/{id}.
//
// Required fields are pointers, so a missing field and an explicit null can
// be told apart from an empty string.
type BookInput struct {
	Title         *string `json:"title"`
	ISBN          *string `json:"isbn"`
	CategoryID    *uint   `json:"category_id"`
	AuthorIDs     []uint  `json:"author_ids"`
	Publisher     *string `json:"publisher"`
	PublishedYear *int    `json:"published_year"`
	Description   *string `json:"description"`
}

// Validate reports every problem found in the input. The returned error
// wraps ErrValidation.
func (in BookInput) Validate() error {
	var problems []string
	if blank(in.Title) {
		problems = append(problems, "title is required")
	}
	if blank(in.ISBN) {
		problems = append(problems, "isbn is required")
	}
	if in.CategoryID == nil || *in.CategoryID == 0 {
		problems = append(problems, "category_id is required")
	}
	if len(in.AuthorIDs) == 0 {
		problems = append(problems, "author_ids must contain at least one id")
	}
	for _, id := range in.AuthorIDs {
		if id == 0 {
			problems = append(problems, "author_ids must contain positive ids")
			break
		}
	}
	if y := in.PublishedYear; y != nil && (*y < minPublishedYear || *y > maxPublishedYear) {
		problems = append(problems, fmt.Sprintf("published_year must be between %d and %d", minPublishedYear, maxPublishedYear))
	}
	if len(problems) > 0 {
		return invalid("%s", strings.Join(problems, "; "))
	}
	return nil
}

// ToBook converts a validated input into a Book (without relations).
func (in BookInput) ToBook() Book {
	return Book{
		Title:         strings.TrimSpace(*in.Title),
		ISBN:          strings.TrimSpace(*in.ISBN),
		CategoryID:    *in.CategoryID,
		Publisher:     in.Publisher,
		PublishedYear: in.PublishedYear,
		Description:   in.Description,
	}
}

// CopyInput is the request body for POST /books/{id}/copies.
type CopyInput struct {
	InventoryNo *string `json:"inventory_no"`
	Status      *string `json:"status"`
}

// Validate requires an inventory number and, when given, a known status.
func (in CopyInput) Validate() error {
	if blank(in.InventoryNo) {
		return invalid("inventory_no is required")
	}
	if in.Status != nil && !ValidCopyStatus(*in.Status) {
		return invalid("status must be one of: available, borrowed, lost, under_repair")
	}
	return nil
}

// ToCopy converts a validated input into a Copy of the given book. A missing
// status defaults to "available".
func (in CopyInput) ToCopy(bookID uint) Copy {
	status := StatusAvailable
	if in.Status != nil {
		status = *in.Status
	}
	return Copy{BookID: bookID, InventoryNo: strings.TrimSpace(*in.InventoryNo), Status: status}
}
