# LibraTrack — Homework 13: gRPC замість REST

Урок 13 «Microservices Architecture: gRPC instead of REST» (Go with GenAI, Модуль 3), LMS: **[ Task 13 ]**.

## Умова (LMS)

| Бали | Що зробити | Де |
|---|---|---|
| 70 | Запустити приклади з презентації на своєму ПК | розділ «Частина 1» |
| +30 | Свої сервер і клієнт на Go: 1 unary RPC + 1 streaming RPC; запустити й показати взаємодію локально | розділ «Частина 2» |
| ** | Перевірити код у SonarQube Cloud, показати результати | розділ «Частина 3» |
| Здати | Скріншоти або відео + посилання на GitHub-репозиторій із застосунками й конфігурацією | розділ «Що здавати» |

---

## Частина 1 — приклади з презентації (70 балів)

Демо-проєкт `libratrack-grpc-demo.zip` вбудовано в модуль `libratrack`: `InventoryService` із 6 RPC
(unary, server/client/bidi streaming), REST-міст grpc-gateway, JWT-автентифікація, mTLS, бенчмарк gRPC vs REST.
Пакети `auth`/`middleware` з демо перейменовано на `grpcauth`/`grpcmw`, щоб не конфліктувати з REST-застосунком.

| Слайди | Що | Команда |
|---|---|---|
| 12–16 | `.proto`, кодогенерація | `proto/inventory.proto`, `make proto` |
| 19–22, 41–42 | unary і три види стрімінгу, клієнт | `go run ./cmd/inventory-server`, `go run ./cmd/inventory-client get copy-1` |
| 28, 43 | REST через grpc-gateway | `go run ./cmd/gateway`, `curl.exe localhost:8080/v1/copies/copy-1` |
| 31–33 | mTLS, токени | `go run ./cmd/gencerts`, `go run ./cmd/inventory-server -tls -auth` |
| 44–46 | чекліст перевірки | `bash scripts/grpc-smoke.sh` → **9/9 OK** |
| 47 | бенчмарк | `go test ./bench -run '^$' -bench=. -benchmem -benchtime=2s` |

Тести: `go test -race -count=1 ./internal/server ./internal/grpcauth ./bench` — ok.

Результати бенчмарку (Windows 11, i9-13900H):

| Бенчмарк | ns/op | allocs/op |
|---|---|---|
| GetCopy, gRPC (послідовно) | 111 127 | 191 |
| GetCopy, ручний REST | 45 617 | 72 |
| GetCopy, REST через gateway | 146 255 | 342 |
| GetCopy, gRPC (паралельно) | 13 309 | 173 |
| GetCopy, REST (паралельно) | 17 894 | 72 |
| Серіалізація Protobuf / JSON | 325 / 933 | 5 / 3 |

Розмір `Copy`: Protobuf 28 Б, JSON 85 Б. Висновок: Protobuf ≈3× менший і ≈2,9× швидший у серіалізації; для одного
малого виклику на loopback gRPC повільніший за простий REST, але під паралельним навантаженням випереджає його.

## Частина 2 — власний сервіс `LoanService` (+30 балів)

Мінімальний власний сервіс у предметній області LibraTrack (видача книг), написаний окремо від демо:

| Файл | Призначення |
|---|---|
| [proto/loan.proto](proto/loan.proto) | контракт `loan.v1`: `CreateLoan` (**unary**) і `WatchLoans` (**server streaming**) |
| [gen/loan/](gen/loan) | згенерований код (`make proto-loan`) |
| [internal/loan/service.go](internal/loan/service.go) | реалізація: валідація → `InvalidArgument`, підписки на нові позики, звільнення при скасуванні |
| [internal/loan/service_test.go](internal/loan/service_test.go) | 6 тестів зі справжнім gRPC-сервером, покриття 90,5% |
| [cmd/loan-server](cmd/loan-server) | сервер `:50061`, лог кожного виклику, reflection |
| [cmd/loan-client](cmd/loan-client) | клієнт: `create` (unary), `watch` (streaming), дедлайн, обробка `io.EOF` і скасування |

Запуск (два термінали):

```bash
go run ./cmd/loan-server                                   # термінал 1
go run ./cmd/loan-client create book-1 reader-1 7          # термінал 2: unary
go run ./cmd/loan-client -max 3 watch reader-1             # термінал 2: стрімінг
go run ./cmd/loan-client create book-3 reader-1            # термінал 3: нова позика з'являється в потоці
go test ./internal/loan -race -count=1 -cover
```

Зафіксований прогін — [docs/evidence/loan-demo.txt](docs/evidence/loan-demo.txt), лог сервера — [docs/evidence/loan-server.log](docs/evidence/loan-server.log).
`watch` віддає наявні позики читача, потім нові в реальному часі; після `max_events` сервер закриває потік (`io.EOF`),
а при `Ctrl+C` клієнта підписка звільняється (є тест).

## Частина 3 — SonarQube Cloud (бонус)

Конфігурація: [sonar-project.properties](sonar-project.properties) і GitHub Action [.github/workflows/sonar.yml](.github/workflows/sonar.yml)
(тести з покриттям → `coverage.out` → сканування). Кроки на стороні користувача:

1. Увійти на <https://sonarcloud.io> через GitHub → **+ → Analyze new project** → імпортувати цей репозиторій.
2. Метод аналізу — **GitHub Actions**; скопіювати токен у GitHub: *Settings → Secrets and variables → Actions → `SONAR_TOKEN`*.
3. Звірити `sonar.organization` і `sonar.projectKey` у `sonar-project.properties` зі значеннями на сторінці проєкту.
4. Запушити в `main` → дочекатися зеленого запуску Action → зробити скриншот Overview (Quality Gate, Bugs, Vulnerabilities, Coverage).

## Що здавати

1. Скріншоти (див. нижче) або відеозапис.
2. Посилання на репозиторій: <https://github.com/Vadim-27/Go-with-GenAI-lessons-13-grpc>.

### Скріншоти

| # | Що показати | Як отримати |
|---|---|---|
| 1 | Тести демо проходять | `go test -race -count=1 ./internal/server ./internal/grpcauth` |
| 2 | Демо: сервер + клієнт + gateway | термінали з `inventory-server`, `inventory-client get copy-1`, `curl.exe localhost:8080/v1/copies/copy-1` |
| 3 | Демо: стрімінг | `inventory-client -max 3 watch copy-1` + `set-status` в іншому терміналі |
| 4 | Бенчмарк | команда бенчмарку, таблиця результатів |
| 5 | **Власний сервіс: unary** | `loan-client create book-1 reader-1 7` + термінал із `loan-server` (видно лог виклику) |
| 6 | **Власний сервіс: streaming** | `loan-client -max 3 watch reader-1`, поруч два `create` в іншому терміналі |
| 7 | Помилка валідації | `loan-client create "" reader-1` → `InvalidArgument` |
| 8 | Тести власного сервісу | `go test ./internal/loan -race -count=1 -cover` |
| 9 | SonarQube Cloud | сторінка Overview проєкту з Quality Gate |
| 10 | GitHub | сторінка репозиторію з файлами |
