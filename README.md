# LibraTrack — Тема 10: веб-інтерфейс, HTMX та один бінарник

Бібліотечний застосунок на Go (Gin, GORM, PostgreSQL): JSON API з теми 9 (bcrypt, JWT, RBAC)
плюс веб-інтерфейс на `html/template` + HTMX без JS-фреймворків. Шаблони, CSS/JS та SQL-міграції
вбудовані через `go:embed`, тому застосунок — це один файл.

## Запуск

```bash
docker compose up -d                      # PostgreSQL на порту 5434
cp .env.example .env                      # задайте JWT_SECRET (мінімум 32 символи)
export $(grep -v '^#' .env | xargs)       # або задайте змінні вручну
go run ./cmd/api                          # міграції (вбудовані) застосовуються при старті
```

* Веб-інтерфейс: <http://localhost:8080/catalog>
* Swagger UI: <http://localhost:8080/swagger>

Перший адміністратор створюється при старті з `BOOTSTRAP_ADMIN_EMAIL` / `BOOTSTRAP_ADMIN_PASSWORD`
(якщо такого email ще немає). Нові користувачі реєструються на `/register` з роллю `reader`.

## Веб-інтерфейс

| Маршрут | Що робить | Доступ |
|---|---|---|
| `GET /catalog?q=` | таблиця книг з індикатором наявності та пошуком | усі |
| `GET /web/books?q=` | HTMX live search: лише рядки `<tr>` (з `HX-Request: true`) | усі |
| `GET/POST /login`, `GET/POST /register`, `POST /logout` | сесія: JWT у cookie `HttpOnly; Secure; SameSite=Strict` | усі |
| `GET /web/books/new`, `POST /web/books` | форма створення книги | `librarian`, `admin` |
| `GET /web/books/:id/edit`, `POST /web/books/:id` | форма редагування | `librarian`, `admin` |
| `DELETE /web/books/:id` | HTMX-видалення рядка: `200` з порожнім тілом | `librarian`, `admin` |
| `GET /static/*` | CSS/JS з `embed.FS` | усі |

Без cookie — `401` (або redirect на `/login`), роль `reader` — `403`, запит з чужого origin — `403`.

HTMX-атрибути (див. `web/templates/catalog.html`, `web/templates/partials/book_row.html`):

```html
<input name="q" hx-get="/web/books" hx-trigger="keyup changed delay:300ms" hx-target="#book-list">
<button hx-delete="/web/books/{{.ID}}" hx-target="closest tr" hx-swap="outerHTML"
        hx-confirm="Delete this book?">Видалити</button>
```

## Один бінарник

```bash
CGO_ENABLED=0 go build -o libratrack ./cmd/api      # Linux/macOS
mkdir -p /tmp/verify_libratrack && cp libratrack /tmp/verify_libratrack/
cd /tmp/verify_libratrack && DATABASE_URL=... JWT_SECRET=... ./libratrack
```

Або в контейнері `FROM scratch` (у образі лише `/libratrack`):

```bash
docker compose --profile app up -d --build           # app на :8080 + postgres
```

| Що вбудовано | Де |
|---|---|
| шаблони (`template.ParseFS`) | `web/templates.go` → `web.TemplatesFS` |
| статика (`fs.Sub(staticFS, "static")`) | `web/static.go` → `web.Static()` |
| SQL-міграції (golang-migrate `iofs`) | `migrations/migrations.go` → `migrations.FS` |
| OpenAPI | `internal/docs/docs.go` |

## DevOps: Vagrant, Floci, k3s

Чекліст і порядок кроків — у [HomeWork_DevOps.md](HomeWork_DevOps.md).

| Файл | Що робить |
|---|---|
| [Vagrantfile](Vagrantfile), [scripts/provision.sh](scripts/provision.sh) | VM Ubuntu 22.04 з Docker, Go, AWS CLI + awslocal, k3s |
| [compose.floci.yaml](compose.floci.yaml) | Floci — емулятор AWS на `localhost:4566` у VM |
| [demos/ec2/run.sh](demos/ec2/run.sh), [demos/lambda/](demos/lambda/) | Demo 1 (EC2) і Demo 2 (Go Lambda) на Floci |
| [k8s/](k8s/) | Postgres + LibraTrack у k3s; `bash /app/k8s/deploy.sh` |

Порти на Windows: `18080` — Compose, `30080` — k3s NodePort, `8088` — Traefik Ingress.

## Тести

```bash
export TEST_DATABASE_URL='postgres://libratrack:libratrack@localhost:5434/libratrack_test?sslmode=disable'
go test -race ./...
```

Тести веб-шару: `internal/handler/web/catalog_test.go` (сторінка каталогу, HTMX-фрагменти, RBAC на `hx-delete`,
CSRF, автоекранування, cookie, вбудована статика). Без `TEST_DATABASE_URL` тести з БД пропускаються.

## Документи

| Документ | Зміст |
|---|---|
| [design_v10.txt](design_v10.txt) | HttpOnly-cookie замість localStorage, аудит шаблонів, перевірка ізоляції бінарника |
| [prompts.txt](prompts.txt) | промпти завдань 2.1 і 2.2 (тема 10) та архів теми 9 |
| [HomeWork 10.md](HomeWork%2010.md) | текст завдання теми 10 |
| [HomeWork_DevOps.md](HomeWork_DevOps.md) | чекліст завдання DevOps: Vagrant, Docker, Floci, k3s |
| [security_design.txt](security_design.txt), [security_audit.md](security_audit.md) | безпека API (тема 9) |

Ролі API: `reader`, `librarian`, `admin`; JSON API приймає лише `Authorization: Bearer`.
