# Промпти для ШІ та рев'ю згенерованого IaC

Принцип уроку: **ШІ швидко пише чернетку, але кожен IAM-блок і кожен ресурс читаєте ви**
(слайди 32–33, 43).

## Промпт 1 · Terraform для функції

```
Згенеруй Terraform (hashicorp/aws ~> 6.0) для AWS Lambda функції на Go:

- runtime provided.al2023, architectures ["x86_64"], handler "bootstrap"
- deployment package: ../build/webhook.zip, з source_code_hash
- timeout 10 секунд, memory 256 МБ
- викликається через API Gateway REST: POST /webhook, інтеграція AWS_PROXY,
  стадія "dev"
- IAM-роль ЛИШЕ з правами на запис у CloudWatch Logs для цієї функції
  (жодних Action = "*", жодних Resource = "*" окрім де це неминуче)
- регіон і ім'я функції винеси у variables
- додай aws_lambda_permission для API Gateway

Окремо поясни, які ресурси створюються, і познач усе, що ти припустив.
```

## Промпт 2 · SAM-шаблон (слайд 33)

```
Generate an AWS SAM template.yaml for a Go Lambda function that:
- is triggered by an API Gateway POST /webhook route
- uses the provided.al2023 runtime, arm64 architecture
- has a 10 second timeout and 256MB memory
- has an IAM policy limited to writing CloudWatch Logs only (no other permissions)

Also generate a local test event JSON for sam local invoke.
```

## Додаток для Floci

Якщо потрібен Terraform саме під емулятор, додайте до промпту:

```
Налаштуй provider "aws" для локального емулятора Floci (http://localhost:4566):
access_key/secret_key = "test", skip_credentials_validation,
skip_metadata_api_check, skip_requesting_account_id = true,
s3_use_path_style = true, і блок endpoints для кожного використаного сервісу.
Ендпоїнт винеси у змінну, щоб прибрати блок для справжньої AWS.
```

## Чекліст рев'ю (друкується на слайді 32)

| # | Питання | Де дивитись | ✔ |
|---|---|---|---|
| 1 | Чи є `Action = "*"` або `Resource = "*"`? Чи кожен дозвіл потрібен функції? | `aws_iam_policy_document` | |
| 2 | Чи збігаються `runtime`/`architectures` зі збіркою (`GOARCH`)? | `aws_lambda_function`, `scripts/build.sh` | |
| 3 | Чи не використано застарілий `go1.x`? | `runtime` | |
| 4 | Чи немає захардкоджених регіону / account ID / імен? | усі `.tf` | |
| 5 | Чи `timeout` = p99 + запас, а не максимум 900? | `timeout` | |
| 6 | Чи пам'ять обґрунтована заміром, а не вгадана? | `memory_size` | |
| 7 | Чи є `aws_lambda_permission` для кожного тригера, і чи звужено `source_arn`? | `aws_lambda_permission` | |
| 8 | Чи прочитано **`terraform plan`** до `apply`? | вивід plan | |

## Вправа: «поганий» результат ШІ

Ось типова «щедра» політика, яку ШІ часто видає за замовчуванням. Знайдіть **усі**
проблеми та перепишіть за зразком `terraform/main.tf`.

```hcl
resource "aws_iam_role_policy" "lambda" {
  role = aws_iam_role.lambda_exec.id
  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [{
      Effect   = "Allow"
      Action   = ["s3:*", "logs:*", "dynamodb:*"]
      Resource = "*"
    }]
  })
}

resource "aws_lambda_function" "webhook" {
  function_name = "webhook"
  runtime       = "go1.x"               # ← ?
  architectures = ["arm64"]             # ← збирали під amd64
  handler       = "main"
  timeout       = 900                   # ← ?
  memory_size   = 3008                  # ← ?
  filename      = "function.zip"
  role          = aws_iam_role.lambda_exec.arn
}
```

<details>
<summary>Відповідь</summary>

1. `s3:*`, `logs:*`, `dynamodb:*` на `Resource = "*"` — функції потрібні лише логи (і, для thumbnail, `GetObject`/`PutObject` у двох конкретних бакетах). DynamoDB взагалі не використовується.
2. `go1.x` знято з підтримки → `provided.al2023` + `handler = "bootstrap"`.
3. `arm64` при збірці під `amd64` → помилка під час виклику; значення мають збігатися.
4. `timeout = 900` — максимум маскує зависання залежності; ставте p99 + запас.
5. `memory_size = 3008` без заміру — підбирайте за «Memory Used».
6. Немає `source_code_hash` — зміни коду не спричинять оновлення функції.
7. Імена та регіон захардкоджені — винести у змінні.
</details>
