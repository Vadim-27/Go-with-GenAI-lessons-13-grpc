# LibraTrack — Homework 12: Go Serverless (Lambda на Floci)

Урок 11 «Go Serverless: Lambda-функції на стероїдах» (Go with GenAI, Модуль 3).
Код і конфігурація — у каталозі [`serverless/`](serverless/) (демо-проєкт уроку `libratrack-lambda-demo.zip` + моя функція `fine`).

## Умова завдання

З LMS ([Tasks 12] Go Serverless) та `homework/README.md` із демо-архіву:

| Бали | Що зробити |
|---|---|
| 70 | Запустити приклади з презентації на власному ПК (пройти всі кроки) |
| +20 | Згенерувати Terraform за допомогою ШІ й проаналізувати його **рядок за рядком** |
| +10 | Написати serverless-функцію з HTTP-тригером (приймає JSON → повертає результат валідації/перетворення) |

**Здати:** скріншоти або відеозапис роботи застосунку **+ посилання на GitHub-репозиторій** із застосунками та конфігураційними файлами.

Вимоги до функції (з `homework/README.md`): змінити предметну область webhook-прикладу; щонайменше дві фікстури в `events/` (валідна й невалідна); `go test` проходить; 400 для поганого вводу, не 502; клієнти й конфігурація — на рівні пакета.
Додатково за шаблоном: таблиця рев'ю Terraform, висновок щодо `terraform plan` на Floci, нотатка «serverless чи постійний сервер» (5–7 речень).

## Що зроблено (статус)

| # | Пункт | Статус | Де |
|---|---|---|---|
| 1 | Запуск прикладів уроку на Floci (`make test / build / deploy-cli / smoke / bench / deploy-tf`) | ✅ виконано, smoke **14/14** | [`serverless/docs/evidence/`](serverless/docs/evidence/) |
| 2 | Terraform від ШІ + аналіз рядок за рядком + `terraform plan` на Floci | ✅ виконано | розділ «Завдання 2» нижче, [`serverless/terraform-ai/`](serverless/terraform-ai/) |
| 3 | Нова serverless-функція з HTTP-тригером: **`fine`** — розрахунок штрафу за прострочення позики | ✅ виконано, покриття 96% | [`serverless/cmd/fine/`](serverless/cmd/fine/) |
| 4 | Нотатка «serverless чи постійний сервер» | ✅ написано | розділ «Завдання 3» |
| 5 | Скріншоти / відео, push на GitHub | ⏳ **за вами** (див. «Що лишилось») | — |

---

## Завдання 1 — Запуск прикладів і функція `fine`

### Функція `fine` (нова предметна область)

`POST /fine` приймає повернення позики й рахує штраф. Гроші — цілі копійки (без `float`), логіка — чиста функція `calculate()` без годинника, тому детермінована й тестується без хмари.

```json
{"loan_id": "L-1001", "category": "standard", "due_date": "2026-10-01", "returned_date": "2026-10-10"}
```

| Категорія | Ставка / день | Ліміт штрафу |
|---|---|---|
| `standard` | 2.00 UAH | 200.00 UAH |
| `new_release` | 5.00 UAH | 500.00 UAH |
| `reference` | 10.00 UAH | 1000.00 UAH |

Відповіді (реальні, з Floci через API Gateway):

```text
200  {"result":{"loan_id":"L-1001","category":"standard","days_overdue":9,"rate_per_day_kopecks":200,"fine_kopecks":1800,"fine":"18.00 UAH","capped":false},"status":"calculated"}
200  {"result":{"loan_id":"L-2","category":"new_release","days_overdue":273,...,"fine":"500.00 UAH","capped":true},"status":"calculated"}
400  {"error":"invalid json"}
400  {"details":["loan_id is required","returned_date must be a valid date in YYYY-MM-DD format"],"error":"validation failed"}
```

Відповідність вимогам:

- **400, не 502** — очікувані помилки клієнта повертаються звичайною відповіддю з `nil`-error ([`handler.go`](serverless/cmd/fine/handler.go)); тест `TestClientErrorsAre400NotPanicNot502` перевіряє 9 сценаріїв (зламаний JSON, порожнє тіло, поганий base64, масив замість об'єкта, невідома категорія, `2026-02-30`, неправильний тип поля…).
- **Усі помилки валідації разом**, а не лише перша (`TestAllValidationProblemsAreReported`).
- **Фікстури** в [`events/`](serverless/events/): `fine_valid.json`, `fine_capped.json` (200); `fine_invalid.json`, `fine_missing_field.json` (400) + `fine_payload_*.json` для `curl`. Вони ж — регресійні тести (`TestEventFixtures`).
- **Конфігурація на рівні пакета** — `logger` (`main.go`) і таблиця `tariffs` (`fine.go`) створюються один раз в init-фазі; клієнтів AWS SDK функція не потребує, тому нічого зайвого в бінарник не потрапило (залежності: лише `aws-lambda-go`).
- Інтегровано в збірку (`scripts/build.sh`), `deploy-cli.sh`, Terraform, SAM-шаблон, `Makefile` і `scripts/test.sh` (секція «2b»).

### Результати запуску

| Перевірка | Результат |
|---|---|
| `make test` (`go vet` + `go test -cover`) | ✅ fine 96.2%, webhook 93.7%, thumbnail 78.9% — [`go-test.txt`](serverless/docs/evidence/go-test.txt) |
| `make build` | ✅ `webhook.zip` 2.5 МБ, `thumbnail.zip` 3.4 МБ, `fine.zip` 2.4 МБ (бінарник `fine` — 5.6 МБ) |
| `make deploy-cli` → `make smoke` | ✅ **14 ✅ / 0 ❌**: прямі виклики, API Gateway, 400-не-502, S3-тригер створив мініатюру — [`smoke-floci.txt`](serverless/docs/evidence/smoke-floci.txt) |
| `make deploy-tf` (apply) → smoke → `destroy` | ✅ 22 ресурси створено, smoke 14/14, повторний `plan` — «No changes», `destroy` 22/22 |
| `sam local` | ⚠️ **не запускав** — SAM CLI не встановлено; `sam/template.yaml` оновлено (FineFunction) і не перевірено |

> Середовище: Windows 11 + Docker Desktop (WSL2). `make`, `zip`, `aws`, `terraform` на хості відсутні, тому скрипти запускав у тимчасовому Linux-контейнері (golang:1.23 + AWS CLI 2.39 + Terraform 1.9.8), підключеному до мережі Floci. Floci 2.3.0, архітектура `x86_64`.

### Холодний старт (крок 4 уроку)

Функція `libratrack-fine`, 10 повторів: перед кожним «холодним» викликом видаляв Lambda-контейнер (`docker rm -f`), далі одразу другий — теплий. Час — `curl` через API Gateway Floci.

| | avg | min | max |
|---|---|---|---|
| COLD | **1199 мс** | 1119 мс | 1258 мс |
| WARM | **13 мс** | 10 мс | 18 мс |

**Застереження «Floci ≠ AWS»** (слайд 44):
- «Cold» на Floci = запуск Docker-контейнера (`public.ecr.aws/lambda/provided:al2023`), а не Firecracker micro-VM. Абсолютні ~1.2 с **не можна** порівнювати зі слайдовими ~120 мс для Go на AWS — у реальній AWS очікую суттєво менше.
- Достовірний лише **відносний** висновок: cold/warm ≈ 90×, а саме виклик у теплому контейнері — ~10 мс. Розмір бінарника (5.6 МБ) на cold у Docker-режимі майже не впливає — домінує старт контейнера.
- Мій перший прогін `scripts/bench-cold-start.sh` дав cold ≈ warm ≈ 480–500 мс і **є недійсним**: Docker-клієнт усередині тимчасового контейнера був надто старий і не видаляв теплі контейнери, а `aws` CLI додає ~450 мс власного старту. Тому цифри вище отримано окремим циклом із хоста через `curl`.
- Другий прогін на вашому ПК (`toolbox/tb.ps1 "make bench"`, функція `libratrack-webhook`, Docker-клієнт уже виправлений) дав **валідний** результат: COLD avg 1700 мс (1642–1784), WARM avg 495 мс (453–581). Обидві цифри містять ~450 мс запуску `aws` CLI, тому важить різниця ≈ 1.2 с — вона збігається з прогоном через `curl` (1199 − 13 ≈ 1.2 с).
- Порівняння з **опублікованим** бенчмарком (Go vs Node/Python для схожого навантаження) я не робив — лише з цифрами зі слайда 14, які автори самі називають «тенденцією, а не специфікацією».
- Збірку без `-ldflags="-s -w"` для порівняння розміру не пробував.

---

## Завдання 2 — Terraform від ШІ: аналіз рядок за рядком

**Промпт** — №1 з [`serverless/docs/ai-prompts.md`](serverless/docs/ai-prompts.md), адаптований під `fine` (provided.al2023, `bootstrap`, REST API `POST /fine`, IAM «лише логи», регіон і ім'я — у variables) + «додаток для Floci». Результат збережено **без правок** як [`serverless/terraform-ai/main.tf`](serverless/terraform-ai/main.tf) (це «до»), фінальна версія після рев'ю — ресурси `fine` у [`serverless/terraform/main.tf`](serverless/terraform/main.tf) (це «після»).

> Чернетку ШІ (Claude) писав той самий агент, що й решту коду; незалежного «чужого» ШІ-результату тут немає — це чесне обмеження. Рев'ю нижче зроблено за чеклістом зі слайда 32 і підтверджено реальним `plan` / `apply` на Floci.

### Таблиця рев'ю

| Ресурс / рядок | Що створює | Ризик | Чи змінили? Чому |
|---|---|---|---|
| `terraform { required_version, aws ~> 6.0 }` | Фіксує версії Terraform і провайдера | Низький: без обмеження версії `apply` на іншій машині міг би дати інший результат | Ні — збігається з `terraform/versions.tf` |
| `provider "aws"`: `access_key="test"`, `skip_*`, `s3_use_path_style`, `endpoints {…}` | Спрямовує провайдер на Floci | Захардкоджені ключі й emulator-налаштування; у реальній AWS `skip_credentials_validation` вимикає перевірку облікових даних | Ні, у чернетці й `terraform/provider.tf` це навмисно (ендпоїнт — змінна `floci_endpoint`). **Для AWS блок `endpoints` та `skip_*` прибрати**, ключі брати зі середовища |
| `variable region / function_name / floci_endpoint` | Параметри замість хардкоду | Низький: вимога чеклісту №4 виконана | Ні |
| `aws_iam_role.lambda_role` + `assume_role_policy` | Роль, яку може взяти лише `lambda.amazonaws.com` | Низький: `Principal` — сервіс, а не `*` | Ні. У фіналі — окрема роль `fine_exec` (див. нижче), а не спільна з S3-функціями |
| `aws_iam_role_policy.lambda_logging`: `Resource = "arn:aws:logs:*:*:*"` | Дозвіл `CreateLogGroup/CreateLogStream/PutLogEvents` | **Середній/високий**: `*:*:*` = усі лог-групи в усіх регіонах акаунта; компрометована функція може писати в чужі групи й створювати довільні | **Так.** У фіналі `arn:aws:logs:*:*:log-group:/aws/lambda/libratrack-fine:*` — лише своя група. `Action *` ШІ не вжив — його стримав явний промпт |
| `aws_cloudwatch_log_group.lambda` (без `retention_in_days`) | Лог-група `/aws/lambda/libratrack-fine` | **Середній**: у плані `retention_in_days = 0` — логи зберігаються вічно й ростуть у вартості. **Фактично зламав `apply`**: група вже існувала (Lambda/Floci створюють її сама на першому виклику) → `ResourceAlreadyExistsException`. `plan` цього **не показав** | **Частково.** Явну групу з фінального `main.tf` не додавав (конвенція демо — не керувати нею). Для продакшену: створювати групу в Terraform **до** першого виклику з `retention_in_days = 14`, а лишки імпортувати (`terraform import`) |
| `aws_lambda_function.fine`: `runtime = "provided.al2023"`, `handler = "bootstrap"` | Функція на кастомному середовищі | Низький: не `go1.x` (знято з підтримки) — слайд 15 | Ні |
| `architectures = ["x86_64"]` (хардкод) | Архітектура функції | **Середній**: має збігатися з `GOARCH` збірки й (у Floci) із хостом, інакше `exec format error` (слайд 32) | **Так.** У фіналі `var.lambda_architecture` (його виставляє `scripts/env.sh` з `uname -m`) |
| `filename = "../build/fine.zip"` | Шлях до пакета | Низький/середній: відносно CWD, а не модуля — зламається, якщо запустити Terraform з іншого каталогу | **Так.** `${path.module}/../build/fine.zip` |
| `source_code_hash = filebase64sha256(…)` | Тригер оновлення функції при зміні коду | Низький: без нього зміни коду не застосовуються | Ні |
| `timeout = 10`, `memory_size = 256` | Ліміти функції | Низький: 10 с розумно (слайд 35). 256 МБ — число скопійоване з прикладу, а не з заміру; функція лише ріже рядки й рахує дати | **Так:** `memory_size = 128` — функція тривіальна. ⚠️ Замір «Memory Used» у справжньому CloudWatch не робив (Floci його не дає) — значення треба підтвердити на AWS |
| `reserved_concurrent_executions = -1` (видно лише в `plan`, у коді його немає) | Без ліміту одночасних виконань | **Середній**: публічний ендпоїнт без throttling може вичерпати ліміт паралельності акаунта (слайд 38) й «зваляти» інші функції | **Ні — ризик прийнято для навчального стенда.** Для продакшену — `reserved_concurrent_executions` + throttling на стадії |
| `depends_on = [aws_cloudwatch_log_group.lambda]` | Порядок створення | Низький сам по собі, але прив'язує функцію до проблемного ресурсу вище | Ні |
| `aws_api_gateway_rest_api / resource / method` | `POST /fine` | **Високий для продакшену**: `authorization = "NONE"` — анонімний публічний endpoint без ключа, authorizer-а чи WAF | **Ні — навмисно** для демо (так само в `webhook`). Для продакшену — API key + usage plan або Cognito/JWT authorizer, WAF |
| `aws_api_gateway_integration`: `type = "AWS_PROXY"`, `integration_http_method = "POST"` | Інтеграція API Gateway → Lambda | Низький: `POST` тут правильний завжди, незалежно від HTTP-методу клієнта (поширена плутанина) | Ні |
| `aws_lambda_permission.apigw`: `source_arn = "<execution_arn>/*/*"` | Дозволяє API Gateway викликати функцію | Низький: звужено до **цього** API, але будь-який stage/метод/шлях | Ні. Можна ще звузити до `/*/POST/fine` (пропущено: Floci це не перевіряє, тож перевірити звуження не зміг би) |
| `aws_api_gateway_deployment` лише з `depends_on` | Розгортання API | **Середній**: без `triggers` наступні зміни маршрутів/інтеграцій **не** перерозгортаються → клієнти бачать застарілий API | **Так.** У фіналі `triggers = sha1(jsonencode([…ресурси fine…]))` і `create_before_destroy` |
| `aws_api_gateway_stage` (`dev`) | Стадія | Низький/середній: немає method settings (throttling, access logs, X-Ray) | Ні (прийнято для демо) |
| `output "invoke_url"` | Готовий URL для `curl` | Низький | Ні |

**Підсумок рев'ю:** з 11 ресурсів чернетки до критичних рішень було чотири — широкий `Resource` у логах, відсутність `triggers` у deployment, хардкод архітектури, неуправлювана лог-група. `Action = "*"` ШІ не вжив: промпт вимагав «лише логи» (слайд 33 — «назвіть точний обсяг IAM»), і це спрацювало.

### `terraform plan` на Floci — чого я **не очікував** побачити

Плани збережено: [чернетка ШІ (11 ресурсів)](serverless/docs/evidence/terraform-plan-ai-draft.txt), [фінальна версія (22 ресурси: webhook + thumbnail + fine)](serverless/docs/evidence/terraform-plan-reviewed.txt).

1. **`retention_in_days = 0`** у лог-групі — у коді цього значення не було; «0» означає «зберігати вічно».
2. **`reserved_concurrent_executions = -1`** — параметр, якого немає в коді, але саме він означає відсутність будь-якого ліміту.
3. **`timeout_milliseconds = 29000`** в інтеграції API Gateway — за замовчуванням 29 с, тобто вдвічі-втричі більше за 10-секундний таймаут Lambda (слайд 35: таймаут не повинен маскувати зависання).
4. **У кожному ресурсі з'явився атрибут `region = "us-east-1"`** — особливість провайдера v6, у коді його не було.
5. **`plan` не побачив жодної проблеми з лог-групою**: «11 to add», зелено. Реально `apply` впав на `ResourceAlreadyExistsException`, бо групу вже створили попередні виклики функції. Висновок: `plan` показує діф конфігурації, а не **стан світу** — «зелений plan» ≠ «apply пройде».
6. Після виправлення групи: `apply` чернетки пройшов, `POST /fine` → 200/400, повторний `plan` — «No changes».
7. **Floci не примусово перевіряє IAM** (розділ 10 README демо): жоден з моїх висновків про IAM не підтверджено емулятором — лише прочитано. Тому звуження ролей — результат рев'ю, а не тесту.

---

## Завдання 3 — Нотатка «serverless чи постійний сервер»

```
Мій проєкт: LibraTrack — бібліотечний застосунок на Go (Gin, GORM, PostgreSQL, JWT/RBAC, HTMX-інтерфейс одним бінарником).
Я б обрав постійний сервер (Docker/Kubernetes — як у k8s/ цього репозиторію), тому що
основний API має передбачуваний щоденний трафік, пул з'єднань із PostgreSQL, сесії в cookie
та HTMX-запити, чутливі до затримки: холодний старт на першому кліку каталогу — це помітна
деградація, а один довготривалий пул до БД у Lambda перетворюється на «шторм з'єднань» (слайд 37).
Але для нерівномірної допоміжної роботи використав би Lambda, тому що вона подієва й рідкісна:
розрахунок штрафу за прострочення (`fine`, нічний прогін за розкладом), мініатюри обкладинок
при завантаженні в S3 (`thumbnail`), приймач вебхуків — платимо лише за виклики, сервер не простоює.
Змішана модель (слайд 28): ядро — на сервері, «краї» — на serverless; рішення приймається для кожного навантаження окремо.
Ризики: не зроблена ідемпотентність (S3 і планувальник доставляють «принаймні один раз» — потрібні ключі),
доступ Lambda до PostgreSQL потребує RDS Proxy або HTTP-шлюзу, вартість стане вищою за сервер
при великому стабільному обсязі викликів (точка беззбитковості — слайд 50), а локальні тести
на Floci не доводять ні поведінки IAM, ні реальних холодних стартів AWS.
```

---

## Як відтворити

Вимоги: Docker 24+, Go 1.22+, AWS CLI v2, `make`, `zip`, Terraform ≥ 1.5. **На Windows — WSL2 (Ubuntu)**, скрипти написано на `bash`.

```bash
cd serverless
make test           # юніт-тести (без Docker)
make up             # Floci (порт 4566) + образ Lambda
make build          # build/{webhook,thumbnail,fine}.zip
make deploy-cli     # розгорнути у Floci через AWS CLI
make smoke          # 14 перевірок
make bench          # cold vs warm (за замовчуванням webhook; fine: scripts/bench-cold-start.sh 10 libratrack-fine)
make cleanup && make plan-tf   # Terraform: лише plan — прочитати!
make deploy-tf      # apply — після читання plan
make down
```

Ручний виклик `fine`:

```bash
source scripts/env.sh
API_ID=$(cat .api-id)
curl -i -X POST "$AWS_ENDPOINT_URL/restapis/$API_ID/dev/_user_request_/fine" -d @events/fine_payload_valid.json
curl -i -X POST "$AWS_ENDPOINT_URL/restapis/$API_ID/dev/_user_request_/fine" -d @events/fine_payload_malformed.json   # 400, не 502
```

> Якщо в WSL/Linux після клонування `scripts/*.sh` дають `Permission denied` — `chmod +x serverless/scripts/*.sh` (на Windows git міг не зберегти біт виконання).

> `scripts/cleanup.sh` не видаляє лог-групи `/aws/lambda/*` — вони лишаються у Floci. Тому, якщо ви керуєте лог-групою в Terraform, спершу видаліть її: `aws --endpoint-url http://localhost:4566 logs delete-log-group --log-group-name /aws/lambda/libratrack-fine`.

## Чекліст перед здачею (з `homework/README.md`)

- [x] `make test` — зелений
- [x] Фікстури валідного та невалідного вводу (`events/fine_*.json`)
- [x] Прочитано кожен рядок згенерованого Terraform, IAM звужено
- [x] Записано час холодного старту та застереження «Floci ≠ AWS»
- [x] Нотатка на 5–7 речень
- [ ] **Скріншоти або відео** (див. нижче)
- [ ] **Push у GitHub і посилання в LMS**

## Що лишилось зробити самостійно

1. **Скріншоти / відео** (у LMS вимагають саме їх; я не міг зняти екран): `make test`, `make smoke` (14 ✅), `curl` на `/fine` з 200 і 400, `terraform plan`, вивід бенчу. Текстові журнали вже лежать у `serverless/docs/evidence/` як запасний доказ.
2. **Закомітити й запушити** (нічого не коміттив і не пушив): `git add serverless "HomeWork 12.md" && git commit && git push`, посилання вставити в LMS.
3. За бажання — прогнати `sam local invoke FineFunction -e events/fine_valid.json` (SAM CLI не було) і підтвердити `memory_size = 128` замірами в справжньому AWS.
