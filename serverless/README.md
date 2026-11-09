# LibraTrack · Go Serverless: Lambda на Floci

Демонстраційний проєкт до **Уроку 11 · Go Serverless: Lambda Functions on Steroids**
(Go with GenAI · Модуль 3). Усе виконується **локально** — без акаунта AWS — на
емуляторі **[Floci](https://floci.io/floci/)**.

Проєкт повторює практичну частину уроку (слайди 39–48): написати Lambda на Go →
протестувати локально → розгорнути через IaC → заміряти холодний старт.

---

## 1. Що всередині і до яких слайдів це належить

| Файл / каталог | Що демонструє | Слайди |
|---|---|---|
| `cmd/webhook/` | HTTP-тригер (API Gateway), валідація, коди 400/200, `json.RawMessage`, структуроване логування | 18, 19, 22, 41 |
| `cmd/thumbnail/` | S3-тригер, цикл по `Records`, клієнт S3 у фазі init, ідемпотентність, «постійна» vs «тимчасова» помилка | 9, 21, 23, 46 |
| `events/*.json` | Фікстури подій (валідна, зламана, без поля, невідома подія, S3) | 20, 42, 45 |
| `sam/template.yaml` + `Makefile` | `sam build` / `sam local invoke` / `sam local start-api` | 20, 42 |
| `terraform/` | Terraform для Lambda + IAM (мінімальні права) + API Gateway + S3-тригер | 30–32, 43 |
| `docs/ai-prompts.md` | Промпти для генерації IaC та **чекліст рев'ю** результату | 32, 33, 43 |
| `scripts/bench-cold-start.sh` | Порівняння cold/warm викликів | 12–16, 44 |
| `scripts/test.sh` | Автоматичний чекліст «що має спрацювати» | 45 |
| `homework/` | Шаблон до домашнього завдання | 48 |

---

## 2. Вимоги

| Інструмент | Версія | Навіщо |
|---|---|---|
| **Docker** + Compose v2 | Docker 24+ | запуск Floci **і** Lambda-контейнерів (Floci запускає їх через Docker) |
| **Go** | 1.22+ | збірка та юніт-тести |
| **AWS CLI v2** | 2.13+ | керування Floci (`aws lambda …`, `aws s3 …`) |
| `zip`, `curl`, `bash` | будь-які | пакування й запити |
| Terraform ≥ 1.5 **або** OpenTofu | опційно | шлях C (IaC) |
| AWS SAM CLI | опційно | шлях A (`sam local`) |

**Windows:** працюйте у WSL2 (Ubuntu) з увімкненою інтеграцією Docker Desktop. Не
запускайте скрипти з PowerShell — вони на `bash`.

Перевірте середовище:

```bash
docker version --format '{{.Server.Version}}'
docker compose version
go version
aws --version
```

---

## 3. Floci за 2 хвилини

- Локальний емулятор AWS з відкритим кодом (MIT), порт **4566**, потрібні лише
  **довільні** облікові дані (`test`/`test`).
- **Lambda виконується у справжніх Docker-контейнерах** (образи
  `public.ecr.aws/lambda/<runtime>`), з warm-пулом — тому холодний старт тут
  *відчувається* як справжній, хоча й не дорівнює AWS (докладніше — розділ 10).
- API Gateway, S3, IAM, CloudWatch Logs, SQS тощо працюють у процесі Floci.
- Тому **потрібен Docker-сокет**: `compose.yaml` монтує `/var/run/docker.sock`.

> ⚠️ Монтування Docker-сокета дає контейнеру Floci доступ до Docker вашої машини.
> Використовуйте це на локальній машині розробника, а не на спільних серверах.

---

## 4. Швидкий старт (≈5 хвилин)

```bash
make test         # 1. юніт-тести (без Docker і без Floci)
make up           # 2. запустити Floci + завантажити образ середовища Lambda
make build        # 3. зібрати build/webhook.zip та build/thumbnail.zip
make deploy-cli   # 4. розгорнути у Floci через AWS CLI
make smoke        # 5. автоматична перевірка
make cleanup      # 6. прибрати за собою
```

Якщо `make smoke` закінчився `✅ … ❌ 0` — усе працює. Далі — покроковий розбір.

---

## 5. Покроковий розбір (відповідає слайдам 40–44)

### Крок 0 · Запуск Floci

```bash
make up
```

Скрипт `scripts/up.sh` виконує `docker compose up -d`, чекає, поки порт 4566
відповість, і **наперед завантажує образ** `public.ecr.aws/lambda/provided:al2023`
(~сотні МБ). Без цього перший виклик на демо «зависне» на кілька хвилин.

Підключіть змінні оточення до **поточного терміналу** (скрипти роблять це самі,
але для ручних команд — потрібно):

```bash
source scripts/env.sh
awsf s3 ls            # порожньо, але без помилки = Floci працює
```

`scripts/env.sh` виставляє `AWS_ENDPOINT_URL=http://localhost:4566`, регіон, ключі
`test/test` і — **важливо** — визначає архітектуру вашого хоста
(`amd64`/`arm64`). Пояснення в розділі 9.

Веб-консоль Floci (необов'язково): <http://localhost:4566/_floci/ui> — показує
створені бакети, функції, API.

### Крок 1 · Написати функцію

Відкрийте `cmd/webhook/` і пройдіться по коду разом із слайдами:

| Місце в коді | Що показати |
|---|---|
| `main.go` | `main()` лише викликає `lambda.Start` (слайд 18); `logger` — на рівні пакета (init-фаза) |
| `handler.go` | сигнатура `func(ctx, events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error)`; помилки клієнта → **звичайна відповідь + `nil`**, а не `error` (інакше 502 — слайд 19) |
| `handler.go` | `json.RawMessage` для `data` — структура залежить від типу події |
| `donation.go` | нормалізація та перевірка ISBN; **усі** помилки валідації разом |
| `handler_test.go`, `fixtures_test.go` | тести як документація поведінки; фікстури з `events/` — регресійні тести |

Запустіть тести — вони не потребують ні Docker, ні Floci:

```bash
make test
```

> 💡 Для демо корисно показати, що логіка тестується **без хмари** — саме тому
> `thumbnail` ховає S3 за інтерфейсом `ObjectStore` (`cmd/thumbnail/store.go`).

### Крок 2 · Локальне тестування

Є три рівні — від найшвидшого до найближчого до продакшену.

#### 2A · `go test` (секунди)

Див. вище. Перевіряє бізнес-логіку.

#### 2B · `sam local` (слайд 20; не потребує Floci)

Потрібні Docker і SAM CLI. SAM збирає функцію через `make build-WebhookFunction`.

```bash
make sam-build
sam local invoke WebhookFunction -e events/valid.json      # очікуємо statusCode 200
sam local invoke WebhookFunction -e events/invalid.json    # очікуємо statusCode 400
sam local start-api                                        # → http://127.0.0.1:3000
curl -X POST http://127.0.0.1:3000/webhook -d @events/payload_valid.json
```

Очікувана відповідь на `valid.json` (приклад):

```json
{"statusCode":200,"headers":{"Content-Type":"application/json","X-Request-Id":"demo-valid-0001"},
 "body":"{\"donation\":{\"title\":\"The Go Programming Language\",\"author\":\"Alan Donovan, Brian Kernighan\",\"isbn\":\"9780134190440\",\"condition\":\"good\",\"donor_email\":\"reader@example.com\"},\"status\":\"processed\"}"}
```

> На Apple Silicon `sam local` запускає `x86_64`-образ через емуляцію — працює,
> але повільніше. Це лише для демонстрації `sam local`; шляхи B і C збирають під
> вашу архітектуру автоматично.

#### 2C · Floci (найближче до реального розгортання)

Тут ми викликаємо функцію так, як це робить AWS — через API Lambda — і додатково
перевіряємо **API Gateway**, **S3-тригер** та **IAM**.

```bash
make build
make deploy-cli
```

Що робить `scripts/deploy-cli.sh` (прочитайте його — це і є «ручний IaC»):

1. створює бакети `libratrack-covers` та `libratrack-thumbnails`;
2. створює IAM-роль із **мінімальною** політикою (логи + `GetObject` з одного
   бакета + `PutObject` в інший) — без `Action: "*"`;
3. створює Lambda `libratrack-webhook` і `libratrack-thumbnail`
   (`provided.al2023`, ваша архітектура, `handler=bootstrap`);
4. створює REST API `POST /webhook` з інтеграцією `AWS_PROXY` і деплоїть стадію `dev`;
5. намагається підключити S3-тригер (`s3:ObjectCreated:*` → `libratrack-thumbnail`).

Ручні виклики:

```bash
source scripts/env.sh

# Пряме викликання функції (слайд 42)
awsf lambda invoke --function-name libratrack-webhook \
  --payload fileb://events/valid.json --cli-binary-format raw-in-base64-out out.json
cat out.json

# Через API Gateway
API_ID=$(cat .api-id)
curl -i -X POST "$AWS_ENDPOINT_URL/restapis/$API_ID/dev/_user_request_/webhook" \
  -d @events/payload_valid.json

# Негативні шляхи — мають бути 400, а не 502 (слайд 45)
curl -i -X POST "$AWS_ENDPOINT_URL/restapis/$API_ID/dev/_user_request_/webhook" \
  -d @events/payload_malformed.json

# S3-тригер: завантажуємо обкладинку → з'являється мініатюра
awsf s3 cp testdata/cover.png s3://libratrack-covers/covers/cover.png
awsf s3 ls s3://libratrack-thumbnails/thumbs/covers/
awsf s3 cp s3://libratrack-thumbnails/thumbs/covers/cover.png ./thumb.png
```

`testdata/cover.png` — 600×900; мініатюра має бути 133×200 (пропорції збережено).

Логи функцій дивіться у логах Floci:

```bash
docker compose logs -f floci          # усе
docker ps --filter name=libratrack    # «теплі» контейнери Lambda
docker logs <container>               # JSON-логи конкретної функції
```

Автоматичний прогін усього вищесказаного:

```bash
make smoke
```

### Крок 3 · IaC: Terraform (слайди 30–33, 43)

Мета кроку — **не просто застосувати**, а *прочитати* згенероване. Тому
`scripts/deploy-tf.sh` за замовчуванням виконує лише `plan`.

```bash
make cleanup          # якщо раніше розгортали через CLI — ресурси мають збігатися за іменами
make build
make plan-tf          # init → validate → plan   (ЧИТАЄМО вивід)
make deploy-tf        # apply — лише після читання plan
terraform -chdir=terraform output
```

> Якщо перед цим ви розгортали через `deploy-cli`, виконайте `make cleanup` —
> інакше Terraform спробує створити ресурси, що вже існують.

**Вправа «рев'ю IaC» (слайд 32)**. Відкрийте `terraform/main.tf` і знайдіть:

1. Політику IAM — чи є `Action = "*"`? (Немає. Кожен дозвіл звужено до ресурсу.)
2. `runtime` та `architectures` — чи збігаються з тим, як зібрано бінарник?
3. Захардкоджені значення (регіон, імена бакетів) — що винесено у `variables.tf`?
4. `timeout`/`memory_size` — чому `10 с / 256 МБ` для webhook і `30 с / 512 МБ`
   для thumbnail? (слайди 35–36)

Далі спробуйте згенерувати такий самий Terraform з ШІ за промптом із
`docs/ai-prompts.md` і **порівняйте** зі своїм: що ШІ зробив ширше, ніж треба?

Провайдер налаштований на Floci у `terraform/provider.tf` (ендпоїнти для кожного
сервісу, `s3_use_path_style`, `skip_*`). У справжній AWS цей блок `endpoints`
та `skip_*` прибирають.

### Крок 4 · Бенчмарк холодних стартів (слайд 44)

```bash
make bench                                  # 10 повторів: cold vs warm
scripts/bench-cold-start.sh 20 libratrack-thumbnail
```

Скрипт видаляє «теплі» контейнери функції (`docker rm -f`) перед кожним
холодним викликом і одразу робить другий — теплий. Приклад виводу:

```
  #1   cold= 1850 мс   warm=  120 мс
  ...
COLD: avg=1700 мс  min=1500 мс  max=2100 мс
WARM: avg=110 мс   min=90 мс    max=160 мс
```

*(Числа ілюстративні — у вас будуть інші.)*

Альтернативний режим, де **кожен** виклик холодний:

```bash
make down && make up-ephemeral
```

**Що записати у висновок** (слайд 44): «cold» на Floci включає запуск
Docker-контейнера, а не Firecracker micro-VM; абсолютні числа не переносяться на
AWS. Корисна — відносна різниця cold/warm та вплив розміру бінарника і пам'яті.
Подивіться розмір пакетів: `ls -lh build/*.zip` (бінарники — одиниці МБ,
слайд 13), і спробуйте зібрати без `-ldflags="-s -w"`, щоб побачити різницю.

---

## 6. Бонус: гаряче перезавантаження (hot-reload) у Floci

Floci вміє запускати код **безпосередньо з каталогу на диску** — без zip і
`update-function-code`. Зручно для «внутрішнього циклу» розробки.

1. Увімкніть у `compose.yaml` (за замовчуванням вимкнено):

   ```yaml
   environment:
     FLOCI_SERVICES_LAMBDA_HOT_RELOAD_ENABLED: "true"
     # необов'язково — дозволені каталоги:
     # FLOCI_SERVICES_LAMBDA_HOT_RELOAD_ALLOWED_PATHS: /home/you/projects
   ```

2. Перезапустіть Floci і зберіть у каталог (не zip): `make build` створює
   `build/webhook/bootstrap`.
3. Створіть функцію з «магічним» бакетом `hot-reload` і **абсолютним шляхом на
   Docker-хості**:

   ```bash
   awsf lambda create-function --function-name webhook-hot \
     --runtime provided.al2023 --handler bootstrap --architectures "$LAMBDA_ARCH" \
     --role arn:aws:iam::000000000000:role/libratrack-lambda-role \
     --code S3Bucket=hot-reload,S3Key="$PWD/build/webhook"
   ```

4. Змінили код → `make build` → одразу викликайте: наступний виклик бере свіжий
   `bootstrap`.

Обмеження: кожен виклик — новий контейнер (немає теплого пулу), а
`update-function-code` перетворює функцію назад на звичайну zip-функцію.

---

## 7. Мережа та адреси

- **Керування (CLI/SDK/Terraform):** `http://localhost:4566`.
- **Виклик REST API:** `http://localhost:4566/restapis/<id>/<stage>/_user_request_/<path>` —
  працює без DNS-налаштувань. Також доступні
  `http://<id>.execute-api.localhost.floci.io:4566/<stage>/<path>` та
  `http://localhost:4566/execute-api/<id>/<stage>/<path>`.
- **S3 — path-style.** У `thumbnail` клієнт S3 вмикає `UsePathStyle`, коли заданий
  `AWS_ENDPOINT_URL`. Floci сам підставляє цю змінну в Lambda-контейнер; також
  усередині контейнерів працює вбудований DNS для `*.localhost.floci.io`.
- Якщо інші контейнери (не з цього проєкту) мають звертатися до Floci по імені —
  розкоментуйте `FLOCI_HOSTNAME: floci` у `compose.yaml`.

---

## 8. Очищення

```bash
make cleanup        # видалити ресурси демо з Floci
make down           # зупинити Floci
rm -rf .floci-data  # забути весь збережений стан
docker ps -aq --filter name=libratrack | xargs -r docker rm -f   # залишкові контейнери Lambda
```

---

## 9. Чому архітектура так важлива (пастка зі слайда 32)

Floci за замовчуванням запускає Lambda-контейнери в архітектурі **Docker-хоста**
(`FLOCI_SERVICES_LAMBDA_HONOUR_ARCHITECTURES=false`). Якщо зібрати бінарник під
`arm64`, а хост — `x86_64` (або навпаки), виклик впаде з помилкою на кшталт
`exec format error`. Тому:

- `scripts/env.sh` визначає архітектуру хоста й експортує `GOARCH_LAMBDA`,
  `LAMBDA_ARCH` та `TF_VAR_lambda_architecture`;
- `scripts/build.sh`, `deploy-cli.sh` і Terraform користуються тими самими значеннями.

Це живий приклад вимоги «*runtime і архітектура в IaC мають збігатися зі збіркою*».
У справжній AWS arm64 (Graviton) зазвичай вигідніший — змінюйте значення явно.

---

## 10. Що Floci робить інакше, ніж AWS

Корисно озвучити студентам — це частина «критичного мислення» уроку.

| Тема | У Floci | Висновок |
|---|---|---|
| Холодний старт | Запуск Docker-контейнера, не Firecracker | Абсолютні мс ≠ AWS; дивіться відносну різницю |
| Provisioned concurrency (слайд 16) | **Не реалізовано** | Продемонструвати можна лише на слайдах/в AWS |
| SnapStart, VpcConfig | Приймаються й зберігаються, **без ефекту** | Не покладайтеся на них для перевірки поведінки |
| IAM-політики | Перевірка дозволів зазвичай не примусова (див. розділ IAM у документації Floci) | Рев'ю політик — **ваша** відповідальність, емулятор не «зловить» `Action: "*"` |
| `apiKeyRequired` в API Gateway | Не примусово | Не тестуйте на цьому автентифікацію ключами |
| Ліміти | Стандартно 128 МБ / 3 с, якщо не задано | Завжди задавайте `timeout` і `memory` явно (ми так і робимо) |
| S3 → Lambda тригер | Спробуйте; якщо не спрацював — `smoke` викличе функцію вручну | Подія в `events/s3_put.json` ідентична за форматом |

---

## 11. Статус перевірки (для викладача)

| Що | Статус |
|---|---|
| Юніт-тести Go (`make test`), `go vet`, `gofmt` | ✅ виконано, проходять; покриття ≈ 94% (webhook), ≈ 78% (thumbnail) |
| Збірка під linux/amd64 та linux/arm64, розмір бінарників | ✅ виконано (≈ 5 МБ webhook, ≈ 8 МБ thumbnail) |
| `scripts/build.sh`: zip із виконуваним `bootstrap` | ✅ виконано |
| Синтаксис `bash`-скриптів, HCL, YAML, JSON | ✅ перевірено |
| Розгортання й виклики на **живому Floci**, `terraform apply`, `sam local`, бенчмарк | ⚠️ **не запускалося** — у середовищі підготовки немає Docker. Команди складено за документацією Floci; **прожени́ть `make smoke` перед заняттям** і зверніть увагу на S3-тригер та Terraform-провайдер — це найімовірніші місця для дрібних розбіжностей із вашою версією Floci |

Версії залежностей Go навмисно закріплено на тих, що збираються Go 1.22; за
бажання оновіть: `go get -u ./... && go mod tidy`.
