#!/usr/bin/env bash
# Підключення: source scripts/env.sh
# Налаштовує AWS CLI / SDK / Terraform на локальний Floci та визначає
# архітектуру, під яку треба збирати Lambda.

export AWS_ENDPOINT_URL="${AWS_ENDPOINT_URL:-http://localhost:4566}"
export AWS_DEFAULT_REGION="${AWS_DEFAULT_REGION:-us-east-1}"
export AWS_REGION="$AWS_DEFAULT_REGION"
export AWS_ACCESS_KEY_ID="${AWS_ACCESS_KEY_ID:-test}"
export AWS_SECRET_ACCESS_KEY="${AWS_SECRET_ACCESS_KEY:-test}"
export AWS_PAGER=""                       # не відкривати pager для виводу CLI

# ── Архітектура ────────────────────────────────────────────────────────────
# ВАЖЛИВО: Floci за замовчуванням запускає Lambda-контейнери в архітектурі
# Docker-хоста (FLOCI_SERVICES_LAMBDA_HONOUR_ARCHITECTURES=false). Тож бінарник
# має збігатися з хостом, інакше виклик впаде з «exec format error».
# Це та сама пастка, що на слайді 32: arm64 у Terraform ≠ збірка під amd64.
case "$(uname -m)" in
  arm64|aarch64) export GOARCH_LAMBDA=arm64 LAMBDA_ARCH=arm64 ;;
  *)             export GOARCH_LAMBDA=amd64 LAMBDA_ARCH=x86_64 ;;
esac
export TF_VAR_lambda_architecture="$LAMBDA_ARCH"

# ── Імена ресурсів ─────────────────────────────────────────────────────────
export WEBHOOK_FN="libratrack-webhook"
export THUMB_FN="libratrack-thumbnail"
export FINE_FN="libratrack-fine"
export COVERS_BUCKET="libratrack-covers"
export THUMBS_BUCKET="libratrack-thumbnails"
export ROLE_NAME="libratrack-lambda-role"
export API_NAME="libratrack-api"
export STAGE="dev"

# Обгортка: явний --endpoint-url працює навіть зі старим AWS CLI (< 2.13).
awsf() { aws --endpoint-url "$AWS_ENDPOINT_URL" "$@"; }
