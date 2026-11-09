-- 000002_seed_demo_data (up)
-- Demo data. No hardcoded ids: rows are linked through unique natural keys.

INSERT INTO categories (name) VALUES
    ('Programming'),
    ('Science Fiction'),
    ('Fantasy');

INSERT INTO authors (full_name, bio) VALUES
    ('Alan Donovan',      'Google engineer, co-author of The Go Programming Language.'),
    ('Brian Kernighan',   'Computer scientist, co-author of The C Programming Language.'),
    ('Robert C. Martin',  'Software engineer and author of Clean Code.'),
    ('Andrew Hunt',       'Co-author of The Pragmatic Programmer.'),
    ('David Thomas',      'Co-author of The Pragmatic Programmer.'),
    ('Martin Fowler',     NULL),
    ('Frank Herbert',     'American science fiction author.'),
    ('Isaac Asimov',      'Prolific science fiction author and biochemist.'),
    ('William Gibson',    NULL),
    ('J. R. R. Tolkien',  'English writer and philologist.');

INSERT INTO books (title, isbn, category_id, publisher, published_year, description)
SELECT v.title, v.isbn, c.id, v.publisher, v.published_year, v.description
FROM (VALUES
    ('The Go Programming Language',      '9780134190440', 'Programming',     'Addison-Wesley', 2015, 'The authoritative Go reference.'),
    ('Clean Code',                       '9780132350884', 'Programming',     'Prentice Hall',  2008, NULL),
    ('The Pragmatic Programmer',         '9780135957059', 'Programming',     'Addison-Wesley', 2019, NULL),
    ('Refactoring',                      '9780134757599', 'Programming',     'Addison-Wesley', 2018, 'Improving the design of existing code.'),
    ('Dune',                             '9780441013593', 'Science Fiction', 'Ace',            1990, NULL),
    ('Foundation',                       '9780553293357', 'Science Fiction', 'Bantam',         1991, NULL),
    ('I, Robot',                         '9780553382563', 'Science Fiction', 'Bantam',         2004, NULL),
    ('Neuromancer',                      '9780441569595', 'Science Fiction', 'Ace',            1984, NULL),
    ('The Hobbit',                       '9780547928227', 'Fantasy',         'Mariner',        2012, NULL),
    ('The Fellowship of the Ring',       '9780547928210', 'Fantasy',         'Mariner',        2012, NULL)
) AS v (title, isbn, category_name, publisher, published_year, description)
JOIN categories c ON c.name = v.category_name;

-- Two books have two authors; two authors (Asimov, Tolkien) have two books.
INSERT INTO book_authors (author_id, book_id)
SELECT a.id, b.id
FROM (VALUES
    ('9780134190440', 'Alan Donovan'),
    ('9780134190440', 'Brian Kernighan'),
    ('9780132350884', 'Robert C. Martin'),
    ('9780135957059', 'Andrew Hunt'),
    ('9780135957059', 'David Thomas'),
    ('9780134757599', 'Martin Fowler'),
    ('9780441013593', 'Frank Herbert'),
    ('9780553293357', 'Isaac Asimov'),
    ('9780553382563', 'Isaac Asimov'),
    ('9780441569595', 'William Gibson'),
    ('9780547928227', 'J. R. R. Tolkien'),
    ('9780547928210', 'J. R. R. Tolkien')
) AS v (isbn, full_name)
JOIN books b   ON b.isbn = v.isbn
JOIN authors a ON a.full_name = v.full_name;

INSERT INTO copies (book_id, inventory_no, status)
SELECT b.id, v.inventory_no, v.status
FROM (VALUES
    ('9780134190440', 'INV-0001', 'available'),
    ('9780134190440', 'INV-0002', 'borrowed'),
    ('9780134190440', 'INV-0003', 'available'),
    ('9780132350884', 'INV-0004', 'available'),
    ('9780132350884', 'INV-0005', 'under_repair'),
    ('9780135957059', 'INV-0006', 'available'),
    ('9780135957059', 'INV-0007', 'borrowed'),
    ('9780134757599', 'INV-0008', 'available'),
    ('9780134757599', 'INV-0009', 'available'),
    ('9780441013593', 'INV-0010', 'available'),
    ('9780441013593', 'INV-0011', 'borrowed'),
    ('9780441013593', 'INV-0012', 'lost'),
    ('9780553293357', 'INV-0013', 'available'),
    ('9780553293357', 'INV-0014', 'available'),
    ('9780553382563', 'INV-0015', 'borrowed'),
    ('9780553382563', 'INV-0016', 'available'),
    ('9780441569595', 'INV-0017', 'available'),
    ('9780441569595', 'INV-0018', 'under_repair'),
    ('9780547928227', 'INV-0019', 'available'),
    ('9780547928227', 'INV-0020', 'borrowed'),
    ('9780547928227', 'INV-0021', 'available'),
    ('9780547928210', 'INV-0022', 'available'),
    ('9780547928210', 'INV-0023', 'borrowed'),
    ('9780547928210', 'INV-0024', 'lost')
) AS v (isbn, inventory_no, status)
JOIN books b ON b.isbn = v.isbn;
