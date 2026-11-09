## Наскрізний проєкт: LibraTrack (Система обліку бібліотеки)

**Часовий ліміт:** до 2 годин.
**Мета:** Захистити REST API **LibraTrack** за промисловими стандартами безпеки. Реалізувати повну автентифікацію з хешуванням паролів (`bcrypt`) та видачею токенів (`JWT`), розробити рольову авторизацію через middleware (`RBAC Middleware`) і провести структурований AI-асистований security-рев'ю кодової бази для виявлення та виправлення вразливостей.

---

### ЧАСТИНА 1: Ручні завдання (архітектура безпеки, моделювання та рев'ю) [~35 хв]
*Цей розділ охоплює завдання, що потребують критичних рішень людини, аналізу ризиків і контролю якості коду.*

* **Завдання 1.1: Ролі, матриця доступу та дизайн DTO (дизайн безпеки та контракту):**
  1. Створіть файл `security_design.txt` (або додайте розділ у `README.md`).
  2. Визначте ролі системи: мінімум 2 ролі (`reader` і `admin`) згідно з завданням (або 3 ролі: `reader`, `librarian`, `admin` для повної моделі capstone).
  3. Опишіть матрицю контролю доступу для ендпоінтів LibraTrack:
     * **Публічні (без автентифікації):** `POST /api/v1/auth/register`, `POST /api/v1/auth/login`, `GET /api/v1/books`.
     * **Захищені (будь-який автентифікований користувач):** `GET /api/v1/me/loans`.
     * **Персонал (`librarian` / `admin`):** `POST /api/v1/books`, `PUT /api/v1/books/:id`, `POST /api/v1/loans`, `POST /api/v1/loans/:id/return`.
     * **Адміністратор (`admin`):** `GET /api/v1/users`, `PUT /api/v1/users/:id/role`.
  4. Спроєктуйте request DTO, що запобігають вразливості Mass Assignment: `RegisterRequest` і `UpdateProfileRequest` повинні містити лише поля, які користувач може редагувати, а до `PasswordHash` додайте тег `json:"-"`, щоб хеш не потрапляв у JSON-відповіді.

* **Завдання 1.2: Ручний аудит обробки паролів і логування:**
  1. Перевірте хендлер `/auth/login`: помилки автентифікації (і невідомий email, і неправильний пароль) повинні повертати однакову HTTP 401 відповідь з повідомленням `"invalid credentials"` — це запобігає User Enumeration (перебору існуючих користувачів).
  2. Перевірте `LoggingMiddleware` з Теми 7: логер запитів повинен приховувати або пропускати поле `password` у тілах запитів.
  3. Перевірте поведінку переривання в middleware: при помилках автентифікації/авторизації обов'язково викликається `c.AbortWithStatusJSON()`, щоб зупинити ланцюжок виконання Gin.

---

### ЧАСТИНА 2: Промпт-інжиніринг (автентифікація, JWT та RBAC middleware) [~40 хв]
*Цей розділ — про написання точних, детально специфікованих промптів для AI-асистента, який генерує код автентифікації та авторизації.*

* **Завдання 2.1: Промпт для модуля автентифікації (bcrypt + видача JWT):**
  * Складіть і виконайте промпт, що генерує файли міграцій БД (`000003_add_auth_fields.up.sql` / `.down.sql`, які додають `password_hash` і `role` до `users`) та оновлює Go-модель `User`.
  * *Вимоги до промпту для хендлерів `Register` і `Login`:*
    1. Реєстрація повинна хешувати паролі через `bcrypt.GenerateFromPassword` з `bcrypt.DefaultCost` (10) і за замовчуванням ставити роль `reader`.
    2. Логін повинен перевіряти пароль через `bcrypt.CompareHashAndPassword` і видавати підписаний JWT з `jwt.SigningMethodHS256`.
    3. Payload JWT повинен містити claims: `user_id`, `role` і час закінчення `exp` (24 години). Секрет підпису береться виключно зі змінної середовища `os.Getenv("JWT_SECRET")`.
  * *Результат:* збережіть текст промпту в `prompts.txt`, а згенеровані файли інтегруйте в `internal/auth/` та `internal/handler/`.

* **Завдання 2.2: Промпт для ланцюжка middleware (`AuthMiddleware` і `RequireRole`):**
  * Напишіть промпт, що генерує `AuthMiddleware` (`internal/middleware/auth.go`) і `RequireRole` (`internal/middleware/rbac.go`).
  * *Вимоги до промпту:*
    1. `AuthMiddleware` витягує токен із заголовка `Authorization: Bearer <token>`, перевіряє підпис і claim `exp`, а claims зберігає в контексті Gin через `c.Set("user_id", ...)` та `c.Set("role", ...)`.
    2. `RequireRole` — variadic-функція (`allowed ...string`), перевіряє `c.Get("role")` і повертає HTTP 401 Unauthenticated (якщо користувач не автентифікований) або HTTP 403 Forbidden (якщо роль не має доступу).
    3. Промпт повинен демонструвати реєстрацію маршрутів у `main.go` через вкладені групи Gin (`public`, `protected`, `staff`, `admin`) з правильним порядком виконання middleware (спочатку Auth, потім RBAC).
  * *Результат:* збережіть промпт у `prompts.txt`.

---

### ЧАСТИНА 3: Агентне використання AI (автоматизований AI-аудит безпеки та тестування) [~45 хв]
*Використайте AI-асистента як автономного партнера для аудиту безпеки та генерації повних тестів.*

* **Завдання 3.1: AI-асистоване security code review:**
  * Надайте AI-агенту код створених модулів (`auth`, `middleware`, `book_handler`, `loan_handler`).
  * *Інструкція для агента:*
    > "Review this Go/Gin API code for security vulnerabilities. Check specifically for: 1) SQL injection (raw queries, string concatenation); 2) Mass assignment (binding request JSON directly to DB models); 3) Missing authentication or RBAC checks; 4) Sensitive data exposure (leaking password hashes in JSON responses). Output a structured list with line numbers, severity (Low/Medium/High), and proposed fixes. Do not rewrite files completely."
  * *Тріаж і звіт:* критично оцініть відповідь AI. Складіть звіт у `security_audit.md` щонайменше з **5 знахідками** та статусами тріажу: *Fixed Now* (виправлено зараз), *Tracked for Later* (відкладено) або *False Positive* (хибне спрацювання).

* **Завдання 3.2: AI-генеровані табличні (table-driven) тести безпеки:**
  * Доручіть AI-агенту написати табличні інтеграційні тести в `auth_test.go` і `rbac_test.go`.
  * *Інструкція для агента:*
    > "Write table-driven integration tests for auth and RBAC using httptest. Cover test cases: 1) Registering duplicate email (HTTP 422); 2) Valid login returning JWT (HTTP 200); 3) Login with wrong password (HTTP 401 generic error); 4) Calling protected route with missing/expired token (HTTP 401); 5) Reader calling POST /api/v1/books (HTTP 403 Forbidden); 6) Admin/Librarian calling POST /api/v1/books (HTTP 201 Created)."
  * *Виконання:* запустіть тести локально командою `go test -v ./...` і переконайтеся, що всі кейси проходять.

---

### Критерії оцінювання та результати (самоперевірка)

Надішліть посилання на репозиторій, що містить:
1. **Міграції БД:** `000003_add_auth_fields.up.sql` та `.down.sql`.
2. **Безпечна обробка паролів:** паролі хешуються через `bcrypt` при реєстрації, `PasswordHash` має тег `json:"-"`.
3. **Дисципліна помилок і токенів:** на `/login` запобігається User Enumeration, JWT містить `exp` і підписується секретом з `os.Getenv("JWT_SECRET")`.
4. **Порядок middleware:** `main.go` чітко розділяє групи маршрутів (`public`, `protected`, `staff`, `admin`), а `RequireRole` виконується після `AuthMiddleware`.
5. **Журнал промптів `prompts.txt`:** містить промпти для завдань 2.1 і 2.2.
6. **Аудит безпеки `security_audit.md`:** документує щонайменше 5 виявлених вразливостей з оцінкою критичності та тріажем виправлень.
7. **Покриття тестами:** інтеграційні тести перевіряють поведінку `401 Unauthenticated` і `403 Forbidden` для ролей `reader`, `librarian` та `admin`.

---
