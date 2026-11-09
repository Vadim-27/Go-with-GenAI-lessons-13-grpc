# Розв'язання проблем

Спершу завжди: **`docker compose logs --tail 100 floci`** — там причина більшості збоїв.

## Запуск Floci

| Симптом | Причина | Що робити |
|---|---|---|
| `Cannot connect to the Docker daemon` | Docker не запущений | Запустіть Docker Desktop / `sudo systemctl start docker` |
| `permission denied … /var/run/docker.sock` | Немає доступу до сокета | У `compose.yaml` вже `user: root`. На Linux додайте себе до групи: `sudo usermod -aG docker $USER` і перелогіньтесь |
| `port is already allocated` (4566) | Інший Floci/LocalStack | `docker ps`, зупиніть зайвий контейнер або змініть порт у `compose.yaml` та `AWS_ENDPOINT_URL` |
| Rootless Podman | Інша мережева схема | Див. розділ «Running on Podman» у документації Floci; можливо потрібен `FLOCI_SERVICES_LAMBDA_DOCKER_HOST_OVERRIDE` |

## Виклик функцій

| Симптом | Причина | Що робити |
|---|---|---|
| Перший виклик «висить» хвилини | Floci тягне образ `public.ecr.aws/lambda/provided:al2023` | `docker pull public.ecr.aws/lambda/provided:al2023` заздалегідь (це робить `make up`). Перевірте доступ до `public.ecr.aws` (VPN/проксі) |
| `exec format error` / `Runtime exited` | Архітектура бінарника ≠ архітектура Docker-хоста | `source scripts/env.sh && make build && make deploy-cli`. Розділ 9 README |
| `connect ECONNREFUSED <ip>:12000` | Lambda-контейнер не бачить Runtime API Floci | Задайте `FLOCI_SERVICES_LAMBDA_DOCKER_HOST_OVERRIDE` (ім'я/IP, доступні з контейнера) у `compose.yaml` |
| `Task timed out after 3.00 seconds` | Не задано `timeout` (типово 3 с) | Задайте `--timeout` / `timeout =`; скрипти це вже роблять |
| Функція повертає зміни кодової бази не одразу | «Теплий» контейнер зі старим кодом | `make deploy-cli` (оновить код), або видаліть контейнер: `docker ps -aq --filter name=libratrack-webhook \| xargs -r docker rm -f` |
| `502 Bad Gateway` від API | Функція повернула `error` або впала в паніку | Дивіться логи контейнера (`docker logs`). Для **очікуваних** помилок повертайте звичайну відповідь з 4xx (слайд 19) |
| `Unable to locate credentials` | Не підключено змінні | `source scripts/env.sh` |
| `Could not connect to the endpoint URL` | Floci не запущено / інший порт | `make up`, `curl -i localhost:4566` |

## S3-тригер

- Перевірте конфігурацію: `awsf s3api get-bucket-notification-configuration --bucket libratrack-covers`.
- Не спрацьовує автоматично? Це нормально для демо — `make smoke` тоді викликає
  `libratrack-thumbnail` вручну з `events/s3_put.json` (формат події ідентичний).
- Ключ у події URL-кодований (`my+cover.jpg`) — функція це враховує (`url.QueryUnescape`).
- `AccessDenied` / `NoSuchBucket` у логах функції — бакети створено? `awsf s3 ls`.

## Terraform

| Симптом | Що робити |
|---|---|
| `Failed to query available provider packages` | `terraform init` потребує Інтернету для завантаження провайдера `hashicorp/aws` |
| `Error: … endpoint … connection refused` для якогось сервісу | Додайте сервіс у блок `endpoints` у `terraform/provider.tf` |
| `already exists` (бакет/роль/функція) | Раніше розгортали через CLI: `make cleanup`, потім `make deploy-tf` |
| `no such file … build/webhook.zip` | `make build` перед Terraform |
| Зміни коду не потрапляють у функцію | `source_code_hash` змінюється разом із zip — перезберіть (`make build`) і повторіть `apply` |

## Go

| Симптом | Що робити |
|---|---|
| `missing go.sum entry` | `go mod tidy` (скрипти роблять це на першому запуску) |
| `requires go >= …` | Оновіть Go до 1.22+ |
| Хочете свіжі версії SDK | `go get -u ./... && go mod tidy`; тоді може знадобитися новіший Go |

## Повне скидання

```bash
make cleanup
make down
rm -rf .floci-data build .api-id terraform/.terraform terraform/terraform.tfstate*
docker ps -aq --filter name=libratrack | xargs -r docker rm -f
docker volume prune --filter label=floci=true
```
