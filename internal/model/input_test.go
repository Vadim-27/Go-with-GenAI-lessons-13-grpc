package model_test

import (
	"errors"
	"strings"
	"testing"

	"libratrack/internal/model"
)

func str(s string) *string { return &s }
func num(n uint) *uint     { return &n }
func year(y int) *int      { return &y }

func validBook() model.BookInput {
	return model.BookInput{
		Title:      str("Dune"),
		ISBN:       str("9780441013593"),
		CategoryID: num(1),
		AuthorIDs:  []uint{1},
	}
}

func TestCategoryInput_Validate(t *testing.T) {
	tests := []struct {
		name string
		in   *string
		ok   bool
	}{
		{"missing", nil, false},
		{"blank", str("   "), false},
		{"one character", str("a"), false},
		{"two characters", str("ab"), true},
		{"forty characters", str(strings.Repeat("a", 40)), true},
		{"forty one characters", str(strings.Repeat("a", 41)), false},
		// Cyrillic letters take two bytes each: the limit counts characters, not bytes.
		{"forty cyrillic characters", str(strings.Repeat(string(rune(0x444)), 40)), true},
		{"forty one cyrillic characters", str(strings.Repeat(string(rune(0x444)), 41)), false},
		{"spaces are trimmed before counting", str(" a "), false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := model.CategoryInput{Name: tt.in}.Validate()
			if tt.ok && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !tt.ok && !errors.Is(err, model.ErrValidation) {
				t.Fatalf("error = %v, want ErrValidation", err)
			}
		})
	}
}

func TestAuthorInput_Validate(t *testing.T) {
	if err := (model.AuthorInput{FullName: str("Frank Herbert")}).Validate(); err != nil {
		t.Errorf("valid author rejected: %v", err)
	}
	for _, name := range []*string{nil, str(""), str("  ")} {
		if err := (model.AuthorInput{FullName: name}).Validate(); !errors.Is(err, model.ErrValidation) {
			t.Errorf("full_name %v: error = %v, want ErrValidation", name, err)
		}
	}
}

func TestBookInput_Validate(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(in *model.BookInput)
		ok     bool
	}{
		{"valid", func(in *model.BookInput) {}, true},
		{"missing title", func(in *model.BookInput) { in.Title = nil }, false},
		{"blank isbn", func(in *model.BookInput) { in.ISBN = str(" ") }, false},
		{"missing category", func(in *model.BookInput) { in.CategoryID = nil }, false},
		{"zero category", func(in *model.BookInput) { in.CategoryID = num(0) }, false},
		{"no authors", func(in *model.BookInput) { in.AuthorIDs = []uint{} }, false},
		{"zero author id", func(in *model.BookInput) { in.AuthorIDs = []uint{1, 0} }, false},
		{"year too small", func(in *model.BookInput) { in.PublishedYear = year(999) }, false},
		{"year too large", func(in *model.BookInput) { in.PublishedYear = year(2101) }, false},
		{"year in range", func(in *model.BookInput) { in.PublishedYear = year(2015) }, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			in := validBook()
			tt.mutate(&in)
			err := in.Validate()
			if tt.ok && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !tt.ok && !errors.Is(err, model.ErrValidation) {
				t.Fatalf("error = %v, want ErrValidation", err)
			}
		})
	}
}

func TestBookInput_ToBook_TrimsAndCopiesFields(t *testing.T) {
	in := validBook()
	in.Title = str("  Dune  ")
	in.Publisher = str("Ace")
	in.PublishedYear = year(1990)

	book := in.ToBook()

	if book.Title != "Dune" {
		t.Errorf("Title = %q, want trimmed value", book.Title)
	}
	if book.CategoryID != 1 || book.ISBN != "9780441013593" {
		t.Errorf("unexpected book: %+v", book)
	}
	if book.Publisher == nil || *book.Publisher != "Ace" || book.PublishedYear == nil || *book.PublishedYear != 1990 {
		t.Errorf("optional fields were not copied: %+v", book)
	}
	if book.Description != nil {
		t.Errorf("Description = %v, want nil (not set)", *book.Description)
	}
}

func TestCopyInput_Validate(t *testing.T) {
	tests := []struct {
		name string
		in   model.CopyInput
		ok   bool
	}{
		{"inventory number only", model.CopyInput{InventoryNo: str("INV-1")}, true},
		{"with valid status", model.CopyInput{InventoryNo: str("INV-1"), Status: str("lost")}, true},
		{"missing inventory number", model.CopyInput{}, false},
		{"blank inventory number", model.CopyInput{InventoryNo: str(" ")}, false},
		{"invalid status", model.CopyInput{InventoryNo: str("INV-1"), Status: str("broken")}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.in.Validate()
			if tt.ok && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !tt.ok && !errors.Is(err, model.ErrValidation) {
				t.Fatalf("error = %v, want ErrValidation", err)
			}
		})
	}
}

func TestCopyInput_ToCopy_DefaultsToAvailable(t *testing.T) {
	c := model.CopyInput{InventoryNo: str(" INV-7 ")}.ToCopy(5)

	if c.BookID != 5 || c.InventoryNo != "INV-7" || c.Status != model.StatusAvailable {
		t.Errorf("unexpected copy: %+v", c)
	}
}

func TestValidCopyStatus(t *testing.T) {
	for _, s := range []string{"available", "borrowed", "lost", "under_repair"} {
		if !model.ValidCopyStatus(s) {
			t.Errorf("%q should be valid", s)
		}
	}
	for _, s := range []string{"", "Available", "broken"} {
		if model.ValidCopyStatus(s) {
			t.Errorf("%q should be invalid", s)
		}
	}
}
