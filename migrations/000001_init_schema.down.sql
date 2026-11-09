-- 000001_init_schema (down)
-- Reverse dependency order.

DROP INDEX IF EXISTS idx_copies_book_id;
DROP INDEX IF EXISTS idx_book_authors_book_id;
DROP INDEX IF EXISTS idx_book_authors_author_id;
DROP INDEX IF EXISTS idx_books_category_id;

DROP TABLE IF EXISTS copies;
DROP TABLE IF EXISTS book_authors;
DROP TABLE IF EXISTS books;
DROP TABLE IF EXISTS users;
DROP TABLE IF EXISTS authors;
DROP TABLE IF EXISTS categories;
