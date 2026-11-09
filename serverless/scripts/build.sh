#!/usr/bin/env bash
# Збирає Lambda-бінарники для provided.al2023 та пакує у zip.
# Використання: scripts/build.sh
set -euo pipefail
cd "$(dirname "$0")/.."
# shellcheck source=env.sh
source scripts/env.sh

command -v go  >/dev/null || { echo "✗ Потрібен Go (https://go.dev/dl/)"; exit 1; }
command -v zip >/dev/null || { echo "✗ Потрібна утиліта zip"; exit 1; }

# Репозиторій постачається без go.sum: перший запуск підтягне залежності.
[[ -f go.sum ]] || { echo "▶ Перший запуск: go mod tidy (завантажує залежності)"; go mod tidy; }

echo "▶ Збірка під linux/${GOARCH_LAMBDA} (Lambda architecture: ${LAMBDA_ARCH})"
rm -rf build && mkdir -p build

for fn in webhook thumbnail fine; do
  mkdir -p "build/${fn}"
  # Кожен прапорець працює на холодний старт (слайд 16):
  #   CGO_ENABLED=0       — статичний бінарник без залежності від libc
  #   -tags lambda.norpc  — прибирає застарілий RPC-режим aws-lambda-go (менший бінарник)
  #   -ldflags "-s -w"    — без таблиці символів і DWARF
  GOOS=linux GOARCH="${GOARCH_LAMBDA}" CGO_ENABLED=0 \
    go build -tags lambda.norpc -ldflags="-s -w" \
      -o "build/${fn}/bootstrap" "./cmd/${fn}"
  # Ім'я `bootstrap` — вимога provided.al2023 (слайд 15).
  (cd "build/${fn}" && zip -q -X "../${fn}.zip" bootstrap)
  size=$(du -h "build/${fn}.zip" | cut -f1)
  echo "  ✓ build/${fn}.zip  (${size})"
done
