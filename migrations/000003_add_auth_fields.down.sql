-- 000003_add_auth_fields (down)
-- Скасування у зворотному порядку: спочатку таблиця (разом з її індексами та
-- обмеженнями), потім індекс на users.

DROP TABLE IF EXISTS loans;
DROP INDEX IF EXISTS idx_users_role;
