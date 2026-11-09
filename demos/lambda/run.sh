#!/usr/bin/env bash
# Demo 2: збірка Go Lambda, деплой у Floci і виклик. Запускати у VM: bash /app/demos/lambda/run.sh
# Повторний запуск оновлює код уже створеної функції.
set -euo pipefail
cd "$(dirname "$0")"
export PATH=$PATH:/usr/local/go/bin

# awslocal (awscli-local) застарів і друкує попередження на кожен виклик;
# це те саме, що aws з endpoint емулятора.
awslocal() { aws --endpoint-url=http://localhost:4566 "$@"; }
# Без автоматичних повторів: інакше повільний холодний старт породжує кілька контейнерів.
export AWS_MAX_ATTEMPTS=1

FN=demo-api
# Стандартні 3 с замало для холодного старту контейнера у VM з 1 CPU.
TIMEOUT=30

# Збірка у VM з 1 CPU (поруч із k3s) дуже повільна, тому bootstrap можна зібрати
# на хості тією самою командою (Go крос-компілює під linux/amd64 з будь-якої ОС):
#   GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -tags lambda.norpc -o bootstrap .
# Якщо готовий bootstrap новіший за main.go, скрипт його використовує.
if [ -f bootstrap ] && [ bootstrap -nt main.go ]; then
  echo "==> Використовую готовий bootstrap (зібраний на хості)"
else
  echo "==> Збірка bootstrap (linux/amd64, статичний)"
  GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -tags lambda.norpc -o bootstrap .
fi
rm -f function.zip
zip -q function.zip bootstrap
ls -la bootstrap function.zip

if awslocal lambda get-function --function-name "$FN" >/dev/null 2>&1; then
  echo "==> Оновлення коду функції $FN"
  awslocal lambda update-function-code --function-name "$FN" --zip-file fileb://function.zip >/dev/null
  awslocal lambda wait function-updated-v2 --function-name "$FN"
  awslocal lambda update-function-configuration --function-name "$FN" --timeout "$TIMEOUT" >/dev/null
else
  echo "==> Створення функції $FN"
  awslocal lambda create-function --function-name "$FN" \
    --runtime provided.al2023 --handler bootstrap --timeout "$TIMEOUT" \
    --zip-file fileb://function.zip \
    --role arn:aws:iam::000000000000:role/lambda >/dev/null
fi

echo "==> Очікування стану Active"
awslocal lambda wait function-active-v2 --function-name "$FN"
awslocal lambda get-function --function-name "$FN" \
  --query 'Configuration.{Name:FunctionName,Runtime:Runtime,State:State,Timeout:Timeout}' --output table

echo "==> Виклик (перший — холодний старт, може тривати до хвилини)"
awslocal lambda invoke --function-name "$FN" --cli-read-timeout 180 \
  --cli-binary-format raw-in-base64-out \
  --payload "{\"name\":\"${1:-Vadym}\"}" /tmp/lambda-out.json >/dev/null
cat /tmp/lambda-out.json
echo
