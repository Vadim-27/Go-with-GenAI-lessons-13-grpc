# Домашнє завдання — DevOps Basics for GenAI Developers

Початок приймання: **вівторок, 20 жовтня 2026**.
Матеріал: презентація «Go with GenAI — DevOps Basics» (Vagrant · Docker · Compose · Terraform · Floci/LocalStack · k3s).

## Текст завдання

> Set up an environment consisting of a virtual machine with Docker installed, Floci running, and a verified K3s
> deployment. Execute the demos for applications 1 and 2 from the presentation. **(70 points)**
>
> \* Containerize the Go application you created. Use Docker Compose to run it alongside a database. **(+30 points)**
>
> \*\* Deploy the application in Kubernetes.
>
> Submit screenshots or a video recording demonstrating the application in operation. Also, provide a link to the
> GitHub repository containing the Docker and Docker Compose configuration files.

Позначки: `[ ]` — не зроблено, `[x]` — зроблено. Скріншоти кладемо в `docs/screenshots/` під іменами з колонки «Доказ».

---

## Частина 1. Середовище: VM + Docker + Floci + k3s (70 балів)

### 1.1 Віртуальна машина (Vagrant, слайди 5–9)

- [x] Встановлено VirtualBox 7.2.22 і Vagrant 2.4.9 на хост (працює поверх Hyper-V)
- [x] Створено `Vagrantfile` у корені репозиторію:
  - [x] box `ubuntu/jammy64` з **зафіксованою версією** `20241002.0.0`
  - [x] пам'ять **3072 МБ**, **1 CPU** (1024 МБ зі слайда замало; Floci і k3s краще не тримати запущеними одночасно)
  - [x] Під Hyper-V (VirtualBox у режимі NEM) Ubuntu з 2 CPU зависала на старті → 1 CPU, `boot_timeout = 900`, `--paravirt-provider none`
  - [x] проброс портів: `8080 → 18080` (app у Compose; 8080 на хості зайнятий іншими проєктами), `30080` (k3s NodePort), `80 → 8088` (Traefik). Floci (`4566`) — лише всередині VM
  - [x] synced folder `.` → `/app`
  - [x] provision-скрипт [scripts/provision.sh](scripts/provision.sh) ідемпотентний (Docker, Go, AWS CLI + awslocal, k3s)
- [x] `.vagrant/` додано в `.gitignore`
- [ ] Перевірено: повторний `vagrant provision` проходить без помилок
- [x] `vagrant up` проходить без помилок (~20 хв уперше), `vagrant ssh` відкриває shell, `/app` змонтовано

| Доказ | Команда |
|---|---|
| ✅ `01-vagrant-status.png` | `vagrant status` → `running` |

### 1.2 Docker у VM

- [x] Provision встановлює Docker Engine 29.9.2 + `docker compose` v5.6.1
- [x] Користувача `vagrant` додано в групу `docker` (працює без `sudo`)
- [x] Перевірка: `docker version` (Engine 29.9.2), `docker compose version` (v5.6.1)

| Доказ | Команда |
|---|---|
| ✅ `02-docker-version.png` | `docker version && docker compose version` |

### 1.3 Floci — емулятор AWS (слайди 29–31)

- [x] Створено [compose.floci.yaml](compose.floci.yaml) (образ `hectorvent/floci:1.5.5`, не `latest` — слайд 38)
- [x] Порт `127.0.0.1:4566:4566`, змонтовано `/var/run/docker.sock` (потрібно для запуску Lambda)
- [x] `docker compose -f /app/compose.floci.yaml up -d`
- [x] Health check відповідає: `curl http://localhost:4566/_localstack/health` → `floci-always-free 1.5.5`, `ec2` і `lambda` — `running`
- [x] AWS CLI v2 і `awslocal` + ключі `test/test`, регіон `eu-central-1` — ставить provision
- [ ] Перевірено у VM: `aws --version && awslocal --version`

| Доказ | Команда |
|---|---|
| ✅ `03-floci-health.png` | `docker ps` + `curl localhost:4566/_localstack/health` |

### 1.4 k3s (слайди 39–40)

- [x] Встановлено k3s `v1.31.4+k3s1` (`curl -sfL https://get.k3s.io | sh -`)
- [x] Kubeconfig скопійовано в `~/.kube/config`, власник — `vagrant`
- [x] Створено namespace `demo`
- [x] `kubectl get nodes` → `devops-hw   Ready   control-plane,master`
- [ ] Тестовий деплой (верифікація кластера): `kubectl create deployment hello --image=nginx:1.27` → под `Running`, потім видалити

| Доказ | Команда |
|---|---|
| ✅ `04-k3s-nodes.png` | `kubectl get nodes -o wide && kubectl get pods -A` |

### 1.5 Demo 1: EC2 на Floci (слайд 32)

Усе одним скриптом: `bash /app/demos/ec2/run.sh` ([demos/ec2/run.sh](demos/ec2/run.sh)).

- [x] `awslocal ec2 create-key-pair --key-name demo`
- [x] `awslocal ec2 describe-images` → 4 образи (amzn2, al2023, ubuntu focal, Windows 2022)
- [x] `awslocal ec2 run-instances --image-id ami-0abcdef1234567890 --instance-type t3.micro --key-name demo`
- [x] `awslocal ec2 describe-instances` → `i-611a2bc9f5891d6b3  t3.micro  running  demo-api`

> `awslocal` застарів і друкує попередження; у скриптах він замінений на `aws --endpoint-url=http://localhost:4566`.

> Емулятор лише імітує метадані EC2, реальна VM не запускається — це нормально.

| Доказ | Команда |
|---|---|
| ✅ `05-demo1-ec2.png` | вивід `run-instances` і `describe-instances` |

### 1.6 Demo 2: Lambda на Floci (слайди 33–36)

Усе одним скриптом: `bash /app/demos/lambda/run.sh` ([demos/lambda/run.sh](demos/lambda/run.sh)).

- [x] Окремий модуль [demos/lambda/](demos/lambda/) (`demo-api`, `aws-lambda-go v1.55.1`, Go 1.26)
- [x] `main.go` з handler `hello, <name> from floci` (слайд 34); локально збирається під linux/amd64
- [x] Збірка: `GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -tags lambda.norpc -o bootstrap .` — **на хості** (у VM з 1 CPU поруч із k3s збірка йшла 30+ хв), `zip function.zip bootstrap` — у VM
- [x] `awslocal lambda create-function --function-name demo-api --runtime provided.al2023 --handler bootstrap --timeout 30 --zip-file fileb://function.zip --role arn:aws:iam::000000000000:role/lambda`
- [x] `awslocal lambda get-function --function-name demo-api` → `State: Active`
- [x] Виклик: `awslocal lambda invoke ... --payload '{"name":"Vadym"}'` → `{"statusCode":200,"body":"hello, Vadym from floci"}`

Що довелося виправити, щоб Lambda запрацювала у Floci:

| Помилка | Причина | Виправлення |
|---|---|---|
| `java.net.BindException: Permission denied` | Floci працює як uid 1001, а `docker.sock` доступний лише групі `docker` | `group_add: ["998"]` у [compose.floci.yaml](compose.floci.yaml) |
| `Function demo-api timed out after 3s` | стандартний тайм-аут 3 с менший за холодний старт контейнера у повільній VM | `--timeout 30`, `--cli-read-timeout 180`, `AWS_MAX_ATTEMPTS=1` |
| `dial tcp 172.18.0.2:9203: i/o timeout` | Floci в мережі Compose (172.18.x), а Lambda-контейнери — у `bridge` (172.17.x), мережі ізольовані | `network_mode: bridge` для Floci |
- [x] `bootstrap` і `function.zip` додано в `.gitignore`

| Доказ | Команда |
|---|---|
| ✅ `06-demo2-lambda.png` | `get-function` + `invoke` з тілом відповіді |

### 1.7 (Опційно) Demo 3: той самий Terraform (слайди 21–28, 37)

- [ ] `terraform/` з `provider.tf` (endpoints → `http://localhost:4566`), IAM-роль, Lambda, S3-бакет
- [ ] У `assume_role_policy` виправити помилки слайда 31: `Version = "2012-10-17"`, `Principal.Service = "lambda.amazonaws.com"`
- [ ] `terraform init && terraform plan && terraform apply -auto-approve`, потім `terraform destroy -auto-approve`
- [ ] `*.tfstate*` і `.terraform/` у `.gitignore`

---

## Частина 2. Контейнеризація LibraTrack + Compose з БД (+30 балів)

Вже є в репозиторії: [Dockerfile](Dockerfile) (multi-stage, `FROM scratch`, non-root) і
[docker-compose.yml](docker-compose.yml) (`postgres` + `app` у профілі `app`).

- [x] Образ зібрано на хості (у VM з 1 CPU збірка Go надто повільна) і передано у VM:
  `docker build -t libratrack:hw10 . && docker save libratrack:hw10 -o libratrack-hw10.tar` → у VM `docker load -i /app/libratrack-hw10.tar`
- [x] `.env` з `JWT_SECRET` (64 символи) та `BOOTSTRAP_ADMIN_*` — **не комітимо** (у `.gitignore`), є лише `.env.example`
- [x] `docker compose --profile app up -d` → обидва сервіси `healthy`, `/catalog` → `200` з VM і з Windows

> Проблема: `app` не підключався до `postgres` (`dial tcp 172.18.0.2:5432: connection timed out`) — k3s переписав iptables
> і стер правила Docker для мережі Compose. Виправлення: `sudo systemctl restart docker` (тепер є в кінці provision).
- [ ] `docker images libratrack` — зафіксувати розмір образу (слайд 16)
- [x] Застосунок відкривається з хоста: <http://localhost:18080/catalog> (8080 у VM → 18080 на Windows), вхід адміністратором, додавання книги
- [x] Застосунок звертається до БД **за іменем сервісу** `postgres`, не `localhost` (слайд 18)
- [x] Логи без помилок: `docker compose logs app`
- [ ] Вправа зі слайда 20: зламати пароль БД → прочитати помилку в логах → повернути
- [ ] Вправа: `docker compose down -v` → `docker volume ls` (volume `pgdata` зник)
- [x] Stretch: `restart: unless-stopped` для обох сервісів
- [x] Stretch: `HEALTHCHECK` у [Dockerfile](Dockerfile) — бінарник з прапорцем `-healthcheck` сам запитує `/health` (у `scratch` немає `curl`)
- [x] `.dockerignore` не пускає в образ `.vagrant/`, `demos/`, `k8s/`, `docs/`; образ `libratrack:hw10` ≈ 26 МБ (зібрано на Windows)
- [ ] (Опційно) профіль `debug` з `adminer` на `8081` (слайд 19)

| Доказ | Команда |
|---|---|
| ✅ `07-compose-ps.png` | `docker compose --profile app ps` |
| ✅ `08-app-catalog.png` | браузер: `/catalog` з книгами |
| ✅ `09-app-logs.png` | `docker compose logs app` (міграції застосовано, сервер слухає `:8080`) |

---

## Частина 3. Деплой у Kubernetes (k3s) (\*\*)

Усе одним скриптом: `bash /app/k8s/deploy.sh` ([k8s/deploy.sh](k8s/deploy.sh)): збірка → імпорт у k3s → Secret → apply → rollout.

- [x] Каталог [k8s/](k8s/) з маніфестами (слайди 41–44), перевірено `kubeconform -strict` (7/7 valid):
  - [x] `00-namespace.yaml` — `libratrack`
  - [x] `10-configmap.yaml` — несекретні змінні (`PORT`, `POSTGRES_USER/DB`, `BOOTSTRAP_ADMIN_EMAIL`)
  - [x] `20-postgres.yaml` — StatefulSet + PVC `local-path` 1Gi + headless Service `postgres`, probes `pg_isready`
  - [x] Secret `libratrack-secrets` створює `deploy.sh` з випадкових паролів, **не в Git**
  - [x] `30-app.yaml` — Deployment (2 репліки, readiness/liveness `/health`, requests/limits, non-root, read-only FS) + Service `NodePort 30080`
  - [x] `40-ingress.yaml` — Traefik без host-правила → <http://localhost:8088/catalog>
- [x] Образ потрапляє в k3s без registry: `docker save | sudo k3s ctr images import -`, `imagePullPolicy: Never`
- [ ] `bash /app/k8s/deploy.sh` → усі поди `Running`, `READY 1/1`
- [ ] <http://localhost:30080/catalog> і <http://localhost:8088/catalog> з Windows повертають сторінку
- [ ] Логін адміністратором `admin@libratrack.local` (пароль — з `.env` або виведений `deploy.sh`)
- [ ] `kubectl -n libratrack scale deploy/libratrack --replicas=3` працює
- [ ] Rolling update + rollback: `bash /app/k8s/deploy.sh hw10.1`, `kubectl -n libratrack rollout history deploy/libratrack`, `kubectl -n libratrack rollout undo deploy/libratrack` (слайд 45)

| Доказ | Команда |
|---|---|
| `10-k8s-pods.png` | `kubectl get pods,svc,ingress -n libratrack` |
| `11-k8s-app.png` | браузер/`curl` до застосунку в k3s |
| `12-k8s-rollout.png` | `kubectl rollout history deploy/libratrack` |

---

## Що здаємо

- [ ] Скріншоти `docs/screenshots/01…12` (або відео з усім ланцюжком)
- [ ] Посилання на GitHub-репозиторій з `Dockerfile`, `docker-compose.yml`, `compose.floci.yaml`, `Vagrantfile`, `k8s/`
- [ ] У `README.md` — розділ «DevOps» з командами запуску
- [ ] Перед комітом: у Git немає `.env`, `*.tfstate`, `bootstrap`, `function.zip`, `.vagrant/`

## Файли

```text
Vagrantfile                 # ✅ VM: 3 ГБ, 1 CPU, порти 18080 / 30080 / 8088
scripts/provision.sh        # ✅ Docker, Go 1.26, AWS CLI + awslocal, k3s, kubeconfig
compose.floci.yaml          # ✅ емулятор AWS (Floci 1.5.5)
demos/ec2/run.sh            # ✅ Demo 1
demos/lambda/               # ✅ Demo 2: Go Lambda + run.sh
Dockerfile                  # ✅ + HEALTHCHECK
docker-compose.yml          # ✅ + restart: unless-stopped
k8s/                        # ✅ маніфести + deploy.sh
terraform/                  # ⬜ Demo 3 (опційно)
docs/screenshots/           # ⬜ докази
```

## Порядок виконання у VM

```bash
vagrant up && vagrant ssh                       # на Windows
docker compose -f /app/compose.floci.yaml up -d # Floci
bash /app/demos/ec2/run.sh                      # Demo 1   → скрін 05
bash /app/demos/lambda/run.sh                   # Demo 2   → скрін 06
docker compose -f /app/compose.floci.yaml down  # звільнити пам'ять перед наступними кроками
cd /app && cp -n .env.example .env              # задати JWT_SECRET (32+ символи)
docker compose --profile app up -d --build      # Частина 2 → скріни 07–09, http://localhost:18080/catalog
docker compose --profile app down               # звільнити пам'ять
bash /app/k8s/deploy.sh                         # Частина 3 → скріни 10–12, http://localhost:30080/catalog
```
