#!/usr/bin/env bash
# Розгортання у Floci через Terraform (шлях C): init → validate → plan → apply.
# Використання: scripts/deploy-tf.sh [plan|apply|destroy]   (за замовчуванням: plan)
#
# Зверніть увагу: за замовчуванням ЛИШЕ plan. Це той самий принцип, що на слайдах
# 32 та 43: читаємо diff, і тільки потім робимо apply.
set -euo pipefail
cd "$(dirname "$0")/.."
source scripts/env.sh
command -v terraform >/dev/null || command -v tofu >/dev/null || { echo "✗ Потрібен Terraform ≥1.5 або OpenTofu"; exit 1; }
TF=$(command -v terraform || command -v tofu)

[[ -f build/webhook.zip ]] || { echo "✗ Спершу: make build"; exit 1; }
cd terraform
mode=${1:-plan}
$TF init -input=false
$TF validate
case "$mode" in
  plan)    $TF plan -input=false
           echo; echo "ℹ️  Це лише plan. Прочитайте його, потім: scripts/deploy-tf.sh apply" ;;
  apply)   $TF apply -input=false -auto-approve
           echo; $TF output ;;
  destroy) $TF destroy -input=false -auto-approve ;;
  *) echo "Невідомий режим: $mode"; exit 1 ;;
esac
