package repository_test

import (
	"context"
	"errors"
	"math"
	"strconv"
	"strings"
	"testing"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"libratrack/internal/model"
	"libratrack/internal/repository"
	"libratrack/internal/testutil"
)

// schemaName is unique to this package, so parallel test packages do not collide.
const schemaName = "t_repository"

var ctx = context.Background()

type fixture struct {
	db         *gorm.DB
	categories repository.CategoryRepository
	authors    repository.AuthorRepository
	books      repository.BookRepository
	copies     repository.CopyRepository
}

func newFixture(t *testing.T) *fixture {
	t.Helper()
	db := testutil.NewDB(t, schemaName)
	return &fixture{
		db:         db,
		categories: repository.NewCategoryRepository(db),
		authors:    repository.NewAuthorRepository(db),
		books:      repository.NewBookRepository(db),
		copies:     repository.NewCopyRepository(db),
	}
}

func (f *fixture) category(t *testing.T, name string) *model.Category {
	t.Helper()
	c := &model.Category{Name: name}
	if err := f.categories.Create(ctx, c); err != nil {
		t.Fatalf("create category %q: %v", name, err)
	}
	return c
}

func (f *fixture) author(t *testing.T, name string) *model.Author {
	t.Helper()
	a := &model.Author{FullName: name}
	if err := f.authors.Create(ctx, a); err != nil {
		t.Fatalf("create author %q: %v", name, err)
	}
	return a
}

func (f *fixture) book(t *testing.T, isbn string, categoryID uint, authorIDs ...uint) *model.Book {
	t.Helper()
	b := &model.Book{Title: "Book " + isbn, ISBN: isbn, CategoryID: categoryID}
	if err := f.books.Create(ctx, b, authorIDs); err != nil {
		t.Fatalf("create book %q: %v", isbn, err)
	}
	return b
}

func count(t *testing.T, db *gorm.DB, table string, where string, args ...any) int64 {
	t.Helper()
	var n int64
	if err := db.Table(table).Where(where, args...).Count(&n).Error; err != nil {
		t.Fatalf("count %s: %v", table, err)
	}
	return n
}

// selectCounter is a GORM logger that only counts SELECT statements.
type selectCounter struct{ selects int }

func (c *selectCounter) LogMode(logger.LogLevel) logger.Interface { return c }
func (c *selectCounter) Info(context.Context, string, ...any)     {}
func (c *selectCounter) Warn(context.Context, string, ...any)     {}
func (c *selectCounter) Error(context.Context, string, ...any)    {}
func (c *selectCounter) Trace(_ context.Context, _ time.Time, fc func() (string, int64), _ error) {
	sql, _ := fc()
	if strings.HasPrefix(strings.ToUpper(strings.TrimSpace(sql)), "SELECT") {
		c.selects++
	}
}

func TestCategoryRepository_CreateAndFind(t *testing.T) {
	f := newFixture(t)

	c := f.category(t, "Programming")
	if c.ID == 0 {
		t.Fatal("ID was not filled after Create")
	}

	got, err := f.categories.FindByID(ctx, c.ID)
	if err != nil || got.Name != "Programming" {
		t.Fatalf("FindByID = %+v, %v", got, err)
	}
}

func TestCategoryRepository_DuplicateName_ReturnsConflict(t *testing.T) {
	f := newFixture(t)
	f.category(t, "Fantasy")

	err := f.categories.Create(ctx, &model.Category{Name: "Fantasy"})

	if !errors.Is(err, repository.ErrConflict) {
		t.Fatalf("error = %v, want ErrConflict", err)
	}
}

func TestCategoryRepository_FindByID_NotFound(t *testing.T) {
	f := newFixture(t)

	_, err := f.categories.FindByID(ctx, 999999)

	if !errors.Is(err, repository.ErrNotFound) {
		t.Fatalf("error = %v, want ErrNotFound", err)
	}
}

func TestAuthorRepository_CreateFindAndList(t *testing.T) {
	f := newFixture(t)
	a := f.author(t, "Frank Herbert")
	f.author(t, "Isaac Asimov")

	got, err := f.authors.FindByID(ctx, a.ID)
	if err != nil || got.FullName != "Frank Herbert" || got.Bio != nil {
		t.Fatalf("FindByID = %+v, %v", got, err)
	}

	list, err := f.authors.List(ctx, 1, 10)
	if err != nil || len(list) != 2 || list[0].ID >= list[1].ID {
		t.Fatalf("List = %+v, %v (want 2 authors ordered by id)", list, err)
	}
}

func TestBookRepository_Create_StoresBookWithRelations(t *testing.T) {
	f := newFixture(t)
	cat := f.category(t, "Programming")
	a1, a2 := f.author(t, "Alan Donovan"), f.author(t, "Brian Kernighan")

	book := f.book(t, "9780134190440", cat.ID, a1.ID, a2.ID, a1.ID) // a1 is repeated on purpose

	if book.ID == 0 || book.Category.Name != "Programming" || len(book.Authors) != 2 {
		t.Fatalf("unexpected book: %+v", book)
	}
	if book.CreatedAt.IsZero() {
		t.Error("CreatedAt was not set")
	}
}

func TestBookRepository_Create_UnknownAuthor_StoresNothing(t *testing.T) {
	f := newFixture(t)
	cat := f.category(t, "Programming")
	a := f.author(t, "Alan Donovan")

	err := f.books.Create(ctx, &model.Book{Title: "T", ISBN: "1", CategoryID: cat.ID}, []uint{a.ID, 999999})

	if !errors.Is(err, repository.ErrInvalidReference) {
		t.Fatalf("error = %v, want ErrInvalidReference", err)
	}
	if n := count(t, f.db, "books", "1 = 1"); n != 0 {
		t.Errorf("books in the database = %d, want 0 (transaction must roll back)", n)
	}
	if n := count(t, f.db, "authors", "1 = 1"); n != 1 {
		t.Errorf("authors in the database = %d, want 1 (no author is created silently)", n)
	}
}

func TestBookRepository_Create_UnknownCategory(t *testing.T) {
	f := newFixture(t)
	a := f.author(t, "Alan Donovan")

	err := f.books.Create(ctx, &model.Book{Title: "T", ISBN: "1", CategoryID: 999999}, []uint{a.ID})

	if !errors.Is(err, repository.ErrInvalidReference) {
		t.Fatalf("error = %v, want ErrInvalidReference", err)
	}
}

func TestBookRepository_Create_DuplicateISBN_ReturnsConflict(t *testing.T) {
	f := newFixture(t)
	cat := f.category(t, "Programming")
	a := f.author(t, "Alan Donovan")
	f.book(t, "9780134190440", cat.ID, a.ID)

	err := f.books.Create(ctx, &model.Book{Title: "Copycat", ISBN: "9780134190440", CategoryID: cat.ID}, []uint{a.ID})

	if !errors.Is(err, repository.ErrConflict) {
		t.Fatalf("error = %v, want ErrConflict", err)
	}
}

func TestBookRepository_FindByID_LoadsRelationsOrReturnsNotFound(t *testing.T) {
	f := newFixture(t)
	cat := f.category(t, "Fantasy")
	a := f.author(t, "J. R. R. Tolkien")
	created := f.book(t, "9780547928227", cat.ID, a.ID)

	got, err := f.books.FindByID(ctx, created.ID)
	if err != nil || got.Category.ID != cat.ID || len(got.Authors) != 1 || got.Authors[0].ID != a.ID {
		t.Fatalf("FindByID = %+v, %v", got, err)
	}

	if _, err := f.books.FindByID(ctx, 999999); !errors.Is(err, repository.ErrNotFound) {
		t.Fatalf("unknown id: error = %v, want ErrNotFound", err)
	}
}

func TestBookRepository_Update_ReplacesFieldsAndAuthors(t *testing.T) {
	f := newFixture(t)
	cat := f.category(t, "Programming")
	other := f.category(t, "Fantasy")
	a1, a2 := f.author(t, "Alan Donovan"), f.author(t, "Brian Kernighan")
	created := f.book(t, "9780134190440", cat.ID, a1.ID)
	description := "new description"

	updated, err := f.books.Update(ctx, created.ID, &model.Book{
		Title: "New title", ISBN: "9780134190440", CategoryID: other.ID, Description: &description,
	}, []uint{a2.ID})
	if err != nil {
		t.Fatal(err)
	}

	if updated.Title != "New title" || updated.Category.ID != other.ID {
		t.Errorf("fields were not replaced: %+v", updated)
	}
	if len(updated.Authors) != 1 || updated.Authors[0].ID != a2.ID {
		t.Errorf("authors = %+v, want only author %d", updated.Authors, a2.ID)
	}
	if n := count(t, f.db, "book_authors", "book_id = ?", created.ID); n != 1 {
		t.Errorf("book_authors rows = %d, want 1", n)
	}
}

func TestBookRepository_Update_NotFound(t *testing.T) {
	f := newFixture(t)
	cat := f.category(t, "Programming")
	a := f.author(t, "Alan Donovan")

	_, err := f.books.Update(ctx, 999999, &model.Book{Title: "T", ISBN: "1", CategoryID: cat.ID}, []uint{a.ID})

	if !errors.Is(err, repository.ErrNotFound) {
		t.Fatalf("error = %v, want ErrNotFound", err)
	}
}

func TestBookRepository_Delete_CascadesToCopiesAndAuthorLinks(t *testing.T) {
	f := newFixture(t)
	cat := f.category(t, "Programming")
	a := f.author(t, "Alan Donovan")
	book := f.book(t, "9780134190440", cat.ID, a.ID)
	if err := f.copies.Create(ctx, &model.Copy{BookID: book.ID, InventoryNo: "INV-1", Status: model.StatusAvailable}); err != nil {
		t.Fatal(err)
	}

	if err := f.books.Delete(ctx, book.ID); err != nil {
		t.Fatal(err)
	}

	if n := count(t, f.db, "copies", "book_id = ?", book.ID); n != 0 {
		t.Errorf("copies left = %d, want 0", n)
	}
	if n := count(t, f.db, "book_authors", "book_id = ?", book.ID); n != 0 {
		t.Errorf("book_authors left = %d, want 0", n)
	}
	if n := count(t, f.db, "authors", "id = ?", a.ID); n != 1 {
		t.Errorf("the author must survive the deletion of the book, authors = %d", n)
	}
}

func TestBookRepository_Delete_NotFound(t *testing.T) {
	f := newFixture(t)

	if err := f.books.Delete(ctx, 999999); !errors.Is(err, repository.ErrNotFound) {
		t.Fatalf("error = %v, want ErrNotFound", err)
	}
}

func TestBookRepository_List_FilterOrderAndPagination(t *testing.T) {
	f := newFixture(t)
	prog, fantasy := f.category(t, "Programming"), f.category(t, "Fantasy")
	a := f.author(t, "Some Author")
	b1 := f.book(t, "1111111111", prog.ID, a.ID)
	f.book(t, "2222222222", fantasy.ID, a.ID)
	b3 := f.book(t, "3333333333", prog.ID, a.ID)

	all, err := f.books.List(ctx, repository.BookFilter{Page: 1, Limit: 10})
	if err != nil || len(all) != 3 || all[0].ID >= all[1].ID || all[1].ID >= all[2].ID {
		t.Fatalf("List = %d books, %v (want 3 ordered by id)", len(all), err)
	}

	programming := prog.ID
	filtered, err := f.books.List(ctx, repository.BookFilter{CategoryID: &programming, Page: 1, Limit: 10})
	if err != nil || len(filtered) != 2 || filtered[0].ID != b1.ID || filtered[1].ID != b3.ID {
		t.Fatalf("filtered List = %+v, %v", filtered, err)
	}

	second, err := f.books.List(ctx, repository.BookFilter{Page: 2, Limit: 2})
	if err != nil || len(second) != 1 || second[0].ID != b3.ID {
		t.Fatalf("page 2 = %+v, %v (want only the third book)", second, err)
	}
}

func TestBookRepository_List_OutOfRangePageIsEmptyNotError(t *testing.T) {
	f := newFixture(t)

	for _, page := range []int{5, math.MaxInt} {
		books, err := f.books.List(ctx, repository.BookFilter{Page: page, Limit: 100})
		if err != nil || books == nil || len(books) != 0 {
			t.Errorf("page %d: books = %v, err = %v (want an empty, non-nil slice)", page, books, err)
		}
	}
}

func TestBookRepository_List_LoadsAuthorsAndCategory(t *testing.T) {
	f := newFixture(t)
	cat := f.category(t, "Science Fiction")
	a1, a2 := f.author(t, "Author One"), f.author(t, "Author Two")
	f.book(t, "1111111111", cat.ID, a1.ID, a2.ID)

	books, err := f.books.List(ctx, repository.BookFilter{Page: 1, Limit: 10})

	if err != nil || len(books) != 1 {
		t.Fatalf("List = %v, %v", books, err)
	}
	if books[0].Category.Name != "Science Fiction" || len(books[0].Authors) != 2 {
		t.Errorf("relations are not loaded: %+v", books[0])
	}
}

// This is the regression test for the N+1 problem: the number of SELECT
// statements of List must not grow with the number of books.
func TestBookRepository_List_NumberOfSelectsDoesNotGrowWithBooks(t *testing.T) {
	f := newFixture(t)
	cat := f.category(t, "Programming")
	a := f.author(t, "Some Author")

	selects := func() int {
		counter := &selectCounter{}
		repo := repository.NewBookRepository(f.db.Session(&gorm.Session{Logger: counter}))
		if _, err := repo.List(ctx, repository.BookFilter{Page: 1, Limit: 100}); err != nil {
			t.Fatal(err)
		}
		return counter.selects
	}

	for i := 0; i < 3; i++ {
		f.book(t, "10000"+strconv.Itoa(i), cat.ID, a.ID)
	}
	few := selects()
	for i := 3; i < 12; i++ {
		f.book(t, "10000"+strconv.Itoa(i), cat.ID, a.ID)
	}
	many := selects()

	if few != many {
		t.Errorf("SELECTs grew from %d (3 books) to %d (12 books): N+1", few, many)
	}
	if many < 1 || many > 6 {
		t.Errorf("SELECTs = %d, want between 1 and 6", many)
	}
}

func TestCopyRepository_CreateListAndAvailability(t *testing.T) {
	f := newFixture(t)
	cat := f.category(t, "Programming")
	a := f.author(t, "Some Author")
	book := f.book(t, "1111111111", cat.ID, a.ID)
	for i, status := range []string{model.StatusAvailable, model.StatusAvailable, model.StatusLost} {
		c := &model.Copy{BookID: book.ID, InventoryNo: "INV-" + strconv.Itoa(i), Status: status}
		if err := f.copies.Create(ctx, c); err != nil || c.ID == 0 {
			t.Fatalf("create copy %d: %v", i, err)
		}
	}

	list, err := f.copies.ListByBook(ctx, book.ID)
	if err != nil || len(list) != 3 {
		t.Fatalf("ListByBook = %d copies, %v", len(list), err)
	}

	av, err := f.copies.Availability(ctx, book.ID)
	if err != nil || av.BookID != book.ID || av.TotalCopies != 3 || av.AvailableCopies != 2 {
		t.Fatalf("Availability = %+v, %v (want total 3, available 2)", av, err)
	}
}

func TestCopyRepository_UnknownBook_ReturnsNotFound(t *testing.T) {
	f := newFixture(t)

	if err := f.copies.Create(ctx, &model.Copy{BookID: 999999, InventoryNo: "X", Status: model.StatusAvailable}); !errors.Is(err, repository.ErrNotFound) {
		t.Errorf("Create: error = %v, want ErrNotFound", err)
	}
	if _, err := f.copies.ListByBook(ctx, 999999); !errors.Is(err, repository.ErrNotFound) {
		t.Errorf("ListByBook: error = %v, want ErrNotFound", err)
	}
	if _, err := f.copies.Availability(ctx, 999999); !errors.Is(err, repository.ErrNotFound) {
		t.Errorf("Availability: error = %v, want ErrNotFound", err)
	}
}

func TestCopyRepository_DuplicateInventoryNo_ReturnsConflict(t *testing.T) {
	f := newFixture(t)
	cat := f.category(t, "Programming")
	a := f.author(t, "Some Author")
	book := f.book(t, "1111111111", cat.ID, a.ID)
	if err := f.copies.Create(ctx, &model.Copy{BookID: book.ID, InventoryNo: "INV-1", Status: model.StatusAvailable}); err != nil {
		t.Fatal(err)
	}

	err := f.copies.Create(ctx, &model.Copy{BookID: book.ID, InventoryNo: "INV-1", Status: model.StatusAvailable})

	if !errors.Is(err, repository.ErrConflict) {
		t.Fatalf("error = %v, want ErrConflict", err)
	}
}

func TestCopyRepository_InvalidStatus_IsRejectedByDatabase(t *testing.T) {
	f := newFixture(t)
	cat := f.category(t, "Programming")
	a := f.author(t, "Some Author")
	book := f.book(t, "1111111111", cat.ID, a.ID)

	err := f.copies.Create(ctx, &model.Copy{BookID: book.ID, InventoryNo: "INV-1", Status: "broken"})

	if err == nil {
		t.Fatal("the CHECK constraint must reject an unknown status")
	}
}
