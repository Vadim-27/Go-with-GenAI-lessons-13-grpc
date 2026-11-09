# Аудит безпеки — LibraTrack, Тема 9 (автентифікація, JWT, RBAC)

**Обсяг:** `internal/auth`, `internal/middleware`, `internal/handler` (auth, user, loan, book, author, category, copy), `internal/repository` (усі), `internal/model` (DTO та моделі), `internal/app` (маршрути), `cmd/api/main.go`, міграції `000001`–`000003`.

**Метод:** AI-агент (Claude Code) переглянув код за інструкцією завдання 3.1: SQL-ін'єкції, mass assignment, відсутні перевірки автентифікації/RBAC, витік чутливих даних; додатково перевірено логування, JWT, гонки при видачі книг, ліміти запитів, CORS. Кожну знахідку перевірено вручну; для виправлених (*Fixed Now*) написано тест, який **падає без виправлення** (перевірено тимчасовим відкатом) і проходить із ним.

**Статуси:** *Fixed Now* — виправлено в цій роботі й підтверджено тестом; *Tracked for Later* — реальний ризик, виправлення заплановане; *False Positive* — підозра не підтвердилась або це свідоме рішення.

> Статуси запропоновано AI-агентом. Остаточне рішення про тріаж ухвалює розробник.

## Зведення

| # | Знахідка | Серйозність | Статус |
|---|---|---|---|
| F1 | SQL-лог GORM записує email і bcrypt-хеші паролів | High | **Fixed Now** |
| F2 | Невідомі поля JSON мовчки ігноруються (Mass Assignment) | Medium | **Fixed Now** |
| F3 | Немає обмеження розміру тіла запиту | Medium | **Fixed Now** |
| F4 | Відповідь логіну з токеном можна кешувати | Low | **Fixed Now** |
| F5 | Немає rate limit на `/auth/login` і `/auth/register` | Medium | Tracked for Later |
| F6 | Роль «зашита» в JWT на 24 години, відкликання немає | Medium | Tracked for Later |
| F7 | Реєстрація розкриває існування email (422 «email already registered») | Low | Tracked for Later |
| F8 | Слабка політика паролів (лише довжина) | Low | Tracked for Later |
| F9 | Немає перевірки `Content-Type` та заголовків безпеки (`nosniff`) | Low | Tracked for Later |
| F10 | Gin довіряє всім проксі (`X-Forwarded-For`) за замовчуванням | Low | Tracked for Later |
| F11 | SQL-ін'єкція в репозиторіях | — | False Positive |
| F12 | Витік `password_hash` у JSON-відповідях | — | False Positive |
| F13 | `Logger` записує пароль із тіла запиту | — | False Positive |
| F14 | Відсутня автентифікація/RBAC на маршрутах | — | False Positive |
| F15 | Гонка при одночасній видачі одного примірника | — | False Positive |
| F16 | CORS `Access-Control-Allow-Origin: *` | — | False Positive |

Разом 16 знахідок: 4 виправлено, 6 відкладено, 6 хибних спрацювань.

## Деталі

### F1 — SQL-лог GORM записує email і bcrypt-хеші · High · Fixed Now
- **Де:** [`internal/repository/db.go`](internal/repository/db.go) рядки 25–37 (`SQLLogger`, раніше конфіг логера всередині `Open`).
- **Ризик:** GORM-логер рівня Info друкує SQL **з підставленими значеннями**. Кожен `INSERT INTO users` і `SELECT ... WHERE email = ...` писав у stdout email та bcrypt-хеш пароля. Хто має доступ до логів (збирач логів, підтримка, лог-агрегатор), отримує хеші для офлайн-підбору.
- **Виправлення:** `ParameterizedQueries: true` — у лог потрапляють плейсхолдери `$1, $2`, а не значення. Логер винесено в `SQLLogger(io.Writer)`, щоб його можна було перевірити.
- **Доказ:** `TestSQLLogger_DoesNotLeakValues` ([`sqllog_test.go`](internal/repository/sqllog_test.go)). Без виправлення тест падає і показує в логу `VALUES ('leak-check@libratrack.com','$2a$10$secretHash...')`.

### F2 — Невідомі поля JSON ігноруються (Mass Assignment) · Medium · Fixed Now
- **Де:** [`internal/handler/errors.go`](internal/handler/errors.go) рядок 89 (`decodeBody`).
- **Ризик:** стандартний `json.Decoder` тихо відкидає зайві ключі. Клієнт, що надсилає `{"role":"admin"}` у реєстрацію чи `{"issued_by":1}` у видачу позики, не отримує помилки, і при будь-якій майбутній зміні DTO (додали поле `role`) це стало б реальною ескалацією прав. Тихе ігнорування також приховує помилки клієнта.
- **Виправлення:** `dec.DisallowUnknownFields()` → 422. Додатково DTO (`RegisterRequest`, `UpdateProfileRequest`, `UpdateRoleRequest`, `LoanInput`) не мають полів `id`, `role`, `password_hash`; роль завжди `reader` (`RegisterRequest.ToUser`).
- **Доказ:** `TestAuthentication_TableDriven/реєстрація: Mass Assignment (role=admin)`, `TestLoans_IssueValidation/Mass Assignment: issued_by у тілі`, `TestProfile_UpdateOnlyFullName`. Без виправлення ці тести падають (перевірено відкатом).

### F3 — Немає обмеження розміру тіла запиту · Medium · Fixed Now
- **Де:** [`internal/handler/errors.go`](internal/handler/errors.go) рядок 85 (`decodeBody`).
- **Ризик:** публічний `POST /auth/register` приймав JSON будь-якого розміру (наприклад, `full_name` на сотні мегабайт) і читав його в пам'ять — просте вичерпання пам'яті (DoS) без автентифікації.
- **Виправлення:** `http.MaxBytesReader` з лімітом 1 МБ (`maxBodyBytes`). Перевищення → 422 «Invalid request body».
- **Доказ:** `TestRequestBody_TooLargeIsRejected` — без виправлення користувач із 2-мегабайтним `full_name` створювався (перевірено відкатом).

### F4 — Відповідь логіну з токеном можна кешувати · Low · Fixed Now
- **Де:** [`internal/handler/auth_handler.go`](internal/handler/auth_handler.go) рядок 98.
- **Ризик:** без `Cache-Control: no-store` проміжний проксі чи браузер може закешувати JWT і віддати його іншому користувачу.
- **Виправлення:** заголовок `Cache-Control: no-store` у відповіді логіну.
- **Доказ:** `TestLogin_ResponseIsNotCacheable` (падає без виправлення).

### F5 — Немає rate limit на логін і реєстрацію · Medium · Tracked for Later
- **Де:** [`internal/app/app.go`](internal/app/app.go) рядки 54–61 (група `public`).
- **Ризик:** необмежений підбір паролів; крім того, bcrypt навмисно повільний, тож потік запитів на `/auth/login` легко перевантажує CPU.
- **План:** middleware rate limit за IP (429 + `Retry-After`) на `/auth/*` і поступове уповільнення на акаунт (не блокування — інакше можна «заблокувати» чужий акаунт). Залежить від F10 (коректний IP клієнта). Вихід за межі завдання теми 9.

### F6 — Роль у JWT діє 24 години, відкликання немає · Medium · Tracked for Later
- **Де:** [`internal/auth/token.go`](internal/auth/token.go) рядок 57 (`TokenTTL`), [`internal/middleware/auth.go`](internal/middleware/auth.go).
- **Ризик:** якщо адмін понизить роль працівника або користувача буде видалено, старий токен зберігає права до `exp` (до 24 год). Викрадений токен не можна відкликати.
- **План:** коротший access-токен (15–60 хв) + refresh-токен із ротацією, або перевірка ролі в БД на кожен запит (`SELECT role FROM users WHERE id = ?`). Це компроміс, зафіксований у `security_design.txt`; для навчального обсягу прийнятний.

### F7 — Реєстрація розкриває існування email · Low · Tracked for Later
- **Де:** [`internal/handler/auth_handler.go`](internal/handler/auth_handler.go) рядок 55.
- **Ризик:** `POST /auth/register` з чужим email відповідає «email already registered» — це дає змогу перевіряти, чи є людина серед користувачів. Поведінка вимагається завданням (422 при дублі), тому логін захищений (однакова відповідь), а реєстрація — ні.
- **План:** підтвердження email: на будь-яку реєстрацію однакова відповідь «лист надіслано».

### F8 — Слабка політика паролів · Low · Tracked for Later
- **Де:** [`internal/model/auth_input.go`](internal/model/auth_input.go) рядок 37.
- **Ризик:** допускається `12345678`. Є лише межі 8–72 байти (72 — ліміт bcrypt).
- **План:** перевірка за списком поширених паролів / оцінка складності.

### F9 — Немає перевірки `Content-Type` і заголовків безпеки · Low · Tracked for Later
- **Де:** [`internal/handler/errors.go`](internal/handler/errors.go), [`internal/app/app.go`](internal/app/app.go).
- **Ризик:** тіло JSON приймається з будь-яким `Content-Type`; немає `X-Content-Type-Options: nosniff`. Ризик низький (API повертає лише JSON, токен — у заголовку, не в cookie).
- **План:** middleware, що вимагає `application/json` на POST/PUT і ставить `nosniff`.

### F10 — Gin довіряє всім проксі · Low · Tracked for Later
- **Де:** [`internal/app/app.go`](internal/app/app.go) (`gin.New()` без `SetTrustedProxies`).
- **Ризик:** `X-Forwarded-For` приймається від кого завгодно, тож IP клієнта можна підробити. Зараз IP ніде не використовується, тому ризик теоретичний, але він стане реальним, щойно з'явиться rate limit (F5).
- **План:** `r.SetTrustedProxies(...)` із списку з конфігурації разом з F5.

### F11 — SQL-ін'єкція в репозиторіях · — · False Positive
- **Де:** усі `Where(...)`/`Order(...)` у [`internal/repository`](internal/repository), напр. [`user_repository.go`](internal/repository/user_repository.go) рядок 44, [`loan_repository.go`](internal/repository/loan_repository.go) рядок 103.
- **Чому не вразливість:** усі значення йдуть через плейсхолдери `?`; `Order(...)` отримує лише константи (`"id ASC"`); `updateColumn` приймає назву колонки тільки з коду (`"role"`, `"full_name"`), а не з вводу. Сирих `Raw`/`Exec` з даними користувача немає (єдині `Exec` — у `testutil`, з константних імен схем).
- **Доказ:** `TestAuthentication_TableDriven/логін: SQL-ін'єкція в email` → 401 (звичайний «невідомий email»).

### F12 — Витік `password_hash` у JSON · — · False Positive
- **Де:** [`internal/model/user.go`](internal/model/user.go) рядок 16.
- **Чому не вразливість:** `PasswordHash` має `json:"-"`; відповіді реєстрації, логіну, `/me`, `/users` серіалізують `model.User`.
- **Доказ:** `TestAuthentication_TableDriven` (реєстрація, логін), `TestAdmin_ChangeRole` (список `/users`) перевіряють відсутність `password_hash`.

### F13 — `Logger` записує пароль із тіла · — · False Positive
- **Де:** [`internal/middleware/logger.go`](internal/middleware/logger.go) рядок 18.
- **Чому не вразливість:** логер пише лише метод, `URL.Path` (без query), статус і тривалість — тіло, query та заголовки не логуються, тож редагувати `password` немає в чому. Зворотний бік: `Authorization` теж не потрапляє в лог.
- **Доказ:** `TestLogger_DoesNotLogSecrets` — пароль із тіла, query-параметри та Bearer-токен відсутні в журналі. Тест додано як захист від регресії. (Справжній витік пароля/хеша в логах був у F1 — через SQL-лог.)

### F14 — Відсутня автентифікація/RBAC на маршрутах · — · False Positive
- **Де:** [`internal/app/app.go`](internal/app/app.go) рядки 54–76.
- **Чому не вразливість:** усі записи каталогу, `/loans*`, `/users*` — у групах `staff`/`admin` (Auth → RequireRole); `/me*` — у `protected`. Незареєстрований маршрут → 404 JSON, а не відкритий доступ. Без `AuthMiddleware` `RequireRole` повертає 401 (fail closed).
- **Доказ:** `TestRBAC_Matrix` (анонім, reader, librarian, admin × ключові ендпоінти), `TestAuthAndRBAC`, `TestRequireRole_WithoutAuthIsUnauthenticated`, `TestRBAC_ForbiddenRequestDoesNotReachHandler`.

### F15 — Гонка при одночасній видачі примірника · — · False Positive
- **Де:** [`internal/repository/loan_repository.go`](internal/repository/loan_repository.go) рядки 41, 71.
- **Чому не вразливість:** видача виконується в транзакції з `SELECT ... FOR UPDATE`, а в БД є частковий унікальний індекс `uq_loans_active_copy` (другий рубіж захисту).
- **Доказ:** `TestLoans_ConcurrentIssueOnlyOnce` — 8 паралельних запитів: рівно один 201, решта 409 (запущено з `-race`).

### F16 — CORS `Access-Control-Allow-Origin: *` · — · False Positive
- **Де:** [`internal/middleware/cors.go`](internal/middleware/cors.go) рядок 14.
- **Чому не вразливість:** автентифікація йде заголовком `Authorization: Bearer`, а не cookie, і `Allow-Credentials` не виставляється. Чужий сайт не може «з'їздити» з сесією користувача. Якщо колись з'являться cookie — потрібен список дозволених origin.

## Свідомі відмінності від завдання

| У завданні | У проєкті | Причина |
|---|---|---|
| Міграція `000003` додає `password_hash` і `role` до `users` | `000003_add_auth_fields` додає `idx_users_role` і таблицю `loans`; колонки вже є в `000001` | Повторний `ADD COLUMN` впав би. Назва файлу збережена |
| Реєстрація груп маршрутів у `main.go` | У `internal/app/app.go` (`NewRouter`), його викликає `main.go` | Одна конфігурація роутів для сервера і тестів |
| Тести в `auth_test.go` / `rbac_test.go` (корінь) | `internal/integration/` на справжньому роутері й PostgreSQL, без моків | Моки перевіряли б не наш код |
| `GET /api/v1/books` — публічний | Також публічні `GET` authors, categories, copies, availability | Це читання каталогу |
| Коди помилок у форматі `UNAUTHENTICATED`/`FORBIDDEN` (як у вихідних тестах) | `unauthenticated`/`forbidden` | Формат кодів проєкту — `snake_case` |
| — | Адмін не може змінити роль собі; `PUT /me` лише `full_name` | Захист від втрати доступу та Mass Assignment |
