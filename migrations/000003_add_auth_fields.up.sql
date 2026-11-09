-- 000003_add_auth_fields (up)
-- Підготовка схеми до автентифікації/авторизації.
--
-- Колонки users.password_hash та users.role (з CHECK на admin/librarian/reader)
-- вже створені у 000001_init_schema, тому тут їх повторно НЕ додаємо.
-- Ця міграція додає те, чого бракує для RBAC-ендпоінтів із завдання:
--   1) індекс по users.role (вибірки користувачів за роллю, GET /users);
--   2) таблицю loans (позики) для POST /loans, POST /loans/:id/return, GET /me/loans.

-- 1) Індекс для фільтрації користувачів за роллю.
CREATE INDEX idx_users_role ON users (role);

-- 2) Позики: який користувач взяв який примірник, ким видано, коли повернути.
CREATE TABLE loans (
    id          BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    user_id     BIGINT      NOT NULL,                      -- читач, який взяв книгу
    copy_id     BIGINT      NOT NULL,                      -- примірник
    issued_by   BIGINT      NOT NULL,                      -- працівник (librarian/admin), який видав
    issued_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    due_at      TIMESTAMPTZ NOT NULL,                      -- крайній термін повернення
    returned_at TIMESTAMPTZ,                               -- NULL = ще не повернуто
    -- RESTRICT: не можна видалити користувача чи примірник з історією позик.
    CONSTRAINT fk_loans_user      FOREIGN KEY (user_id)   REFERENCES users (id)  ON DELETE RESTRICT,
    CONSTRAINT fk_loans_copy      FOREIGN KEY (copy_id)   REFERENCES copies (id) ON DELETE RESTRICT,
    CONSTRAINT fk_loans_issued_by FOREIGN KEY (issued_by) REFERENCES users (id)  ON DELETE RESTRICT,
    CONSTRAINT ck_loans_due_after_issue CHECK (due_at > issued_at),
    CONSTRAINT ck_loans_return_after_issue CHECK (returned_at IS NULL OR returned_at >= issued_at)
);

-- Індекси на зовнішні ключі (JOIN-и та вибірка "мої позики").
CREATE INDEX idx_loans_user_id   ON loans (user_id);
CREATE INDEX idx_loans_copy_id   ON loans (copy_id);
CREATE INDEX idx_loans_issued_by ON loans (issued_by);

-- Один примірник не може бути виданий двічі одночасно:
-- унікальність лише серед активних (неповернених) позик.
CREATE UNIQUE INDEX uq_loans_active_copy ON loans (copy_id) WHERE returned_at IS NULL;
