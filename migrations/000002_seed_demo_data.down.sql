-- 000002_seed_demo_data (down)
-- Removes only the seeded rows, in reverse dependency order.

DELETE FROM copies
WHERE inventory_no IN (
    'INV-0001', 'INV-0002', 'INV-0003', 'INV-0004', 'INV-0005', 'INV-0006',
    'INV-0007', 'INV-0008', 'INV-0009', 'INV-0010', 'INV-0011', 'INV-0012',
    'INV-0013', 'INV-0014', 'INV-0015', 'INV-0016', 'INV-0017', 'INV-0018',
    'INV-0019', 'INV-0020', 'INV-0021', 'INV-0022', 'INV-0023', 'INV-0024'
);

DELETE FROM book_authors
WHERE book_id IN (
    SELECT id FROM books WHERE isbn IN (
        '9780134190440', '9780132350884', '9780135957059', '9780134757599',
        '9780441013593', '9780553293357', '9780553382563', '9780441569595',
        '9780547928227', '9780547928210'
    )
);

DELETE FROM books
WHERE isbn IN (
    '9780134190440', '9780132350884', '9780135957059', '9780134757599',
    '9780441013593', '9780553293357', '9780553382563', '9780441569595',
    '9780547928227', '9780547928210'
);

DELETE FROM authors
WHERE full_name IN (
    'Alan Donovan', 'Brian Kernighan', 'Robert C. Martin', 'Andrew Hunt',
    'David Thomas', 'Martin Fowler', 'Frank Herbert', 'Isaac Asimov',
    'William Gibson', 'J. R. R. Tolkien'
);

DELETE FROM categories
WHERE name IN ('Programming', 'Science Fiction', 'Fantasy');
