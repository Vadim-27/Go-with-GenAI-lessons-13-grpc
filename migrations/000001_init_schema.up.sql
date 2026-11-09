-- 000001_init_schema (up)
-- Normalized (3NF) schema for LibraTrack.

CREATE TABLE categories (
    id   BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    name TEXT NOT NULL,
    CONSTRAINT uq_categories_name UNIQUE (name),
    CONSTRAINT ck_categories_name_len CHECK (char_length(btrim(name)) BETWEEN 2 AND 40)
);

CREATE TABLE authors (
    id        BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    full_name TEXT NOT NULL,
    bio       TEXT
);

CREATE TABLE books (
    id             BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    title          TEXT        NOT NULL,
    isbn           TEXT        NOT NULL,
    category_id    BIGINT      NOT NULL,
    publisher      TEXT,
    published_year INTEGER,
    description    TEXT,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT uq_books_isbn UNIQUE (isbn),
    CONSTRAINT fk_books_category FOREIGN KEY (category_id)
        REFERENCES categories (id) ON DELETE RESTRICT,
    CONSTRAINT ck_books_published_year
        CHECK (published_year IS NULL OR published_year BETWEEN 1000 AND 2100)
);

-- Many-to-Many join table between books and authors.
CREATE TABLE book_authors (
    author_id BIGINT NOT NULL,
    book_id   BIGINT NOT NULL,
    PRIMARY KEY (author_id, book_id),
    CONSTRAINT fk_book_authors_author FOREIGN KEY (author_id)
        REFERENCES authors (id) ON DELETE RESTRICT,
    CONSTRAINT fk_book_authors_book FOREIGN KEY (book_id)
        REFERENCES books (id) ON DELETE CASCADE
);

CREATE TABLE copies (
    id           BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    book_id      BIGINT NOT NULL,
    inventory_no TEXT   NOT NULL,
    status       TEXT   NOT NULL DEFAULT 'available',
    CONSTRAINT uq_copies_inventory_no UNIQUE (inventory_no),
    CONSTRAINT fk_copies_book FOREIGN KEY (book_id)
        REFERENCES books (id) ON DELETE CASCADE,
    CONSTRAINT ck_copies_status
        CHECK (status IN ('available', 'borrowed', 'lost', 'under_repair'))
);

CREATE TABLE users (
    id            BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    email         TEXT        NOT NULL,
    password_hash TEXT        NOT NULL,
    full_name     TEXT,
    role          TEXT        NOT NULL DEFAULT 'reader',
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT uq_users_email UNIQUE (email),
    CONSTRAINT ck_users_role CHECK (role IN ('admin', 'librarian', 'reader'))
);

-- Indexes on foreign keys (filters, joins, preloading).
CREATE INDEX idx_books_category_id      ON books (category_id);
CREATE INDEX idx_book_authors_author_id ON book_authors (author_id);
CREATE INDEX idx_book_authors_book_id   ON book_authors (book_id);
CREATE INDEX idx_copies_book_id         ON copies (book_id);
