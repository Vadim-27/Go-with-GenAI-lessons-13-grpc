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

Чекліст і порядок кроків — в історії git (файл `HomeWork_DevOps.md` замінено завданням теми 13).

| Файл | Що робить |
|---|---|
| [Vagrantfile](Vagrantfile), [scripts/provision.sh](scripts/provision.sh) | VM Ubuntu 22.04 з Docker, Go, AWS CLI + awslocal, k3s |
| [compose.floci.yaml](compose.floci.yaml) | Floci — емулятор AWS на `localhost:4566` у VM |
| [demos/ec2/run.sh](demos/ec2/run.sh), [demos/lambda/](demos/lambda/) | Demo 1 (EC2) і Demo 2 (Go Lambda) на Floci |
| [k8s/](k8s/) | Postgres + LibraTrack у k3s; `bash /app/k8s/deploy.sh` |

Порти на Windows: `18080` — Compose, `30080` — k3s NodePort, `8088` — Traefik Ingress.

## gRPC (тема 13): LoanService і InventoryService

**Власний сервіс (завдання +30):** `proto/loan.proto` — `CreateLoan` (unary) і `WatchLoans` (server streaming); `cmd/loan-server` (:50061), `cmd/loan-client`, `internal/loan`.

```bash
go run ./cmd/loan-server
go run ./cmd/loan-client create book-1 reader-1 7
go run ./cmd/loan-client -max 3 watch reader-1
go test ./internal/loan -race -count=1 -cover
```

**Демо з презентації — InventoryService:**

Окремий gRPC-сервіс обліку примірників (`Copy`) поруч із REST API: контракт `proto/inventory.proto`, згенерований код
у `gen/inventory/`, сервер `cmd/inventory-server` (:50051), клієнт `cmd/inventory-client`, REST-міст `cmd/gateway` (:8080),
mTLS (`cmd/gencerts`), JWT (`cmd/mint-token`), бенчмарк `bench/`. Дані зберігаються в пам'яті (БД не потрібна).
Деталі та результати — у [HomeWork 13.md](HomeWork%2013.md).

```bash
go run ./cmd/inventory-server                  # термінал 1
go run ./cmd/inventory-client get copy-1       # термінал 2 (також: register, set-status, watch, bulk, sync)
go run ./cmd/gateway                           # термінал 3 → curl localhost:8080/v1/copies/copy-1
go run ./cmd/inventory-server -auth            # + JWT: токен через go run ./cmd/mint-token -sub anna -role librarian
go run ./cmd/gencerts && go run ./cmd/inventory-server -tls -auth   # mTLS
go test ./internal/server ./internal/grpcauth ./bench -race -count=1
go test ./bench -run "^$" -bench=. -benchmem   # gRPC vs REST
bash scripts/grpc-smoke.sh                     # наскрізна перевірка
make proto                                     # перегенерація (потрібні protoc і плагіни; make tools)
```

Набір промптів для роботи з ШІ над gRPC — [docs/grpc-prompt-kit/](docs/grpc-prompt-kit/README.md).

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
| [HomeWork 13.md](HomeWork%2013.md) | завдання теми 13 (gRPC): що зроблено, результати тестів і бенчмарку |
| [prompts.txt](prompts.txt) | журнал промптів теми 13 (відповідність промптів з [docs/grpc-prompt-kit](docs/grpc-prompt-kit/README.md)) |
| [security_design.txt](security_design.txt), [security_audit.md](security_audit.md) | безпека API (тема 9) |

Ролі API: `reader`, `librarian`, `admin`; JSON API приймає лише `Authorization: Bearer`.
