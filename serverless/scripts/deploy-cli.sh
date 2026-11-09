#!/usr/bin/env bash
# Розгортання у Floci «вручну» через AWS CLI (шлях B).
# Ідемпотентний: повторний запуск оновлює код і конфігурацію.
set -euo pipefail
cd "$(dirname "$0")/.."
source scripts/env.sh

[[ -f build/webhook.zip && -f build/thumbnail.zip && -f build/fine.zip ]] || { echo "✗ Спершу: make build"; exit 1; }

ACCOUNT_ID="000000000000"   # FLOCI_DEFAULT_ACCOUNT_ID за замовчуванням
ROLE_ARN="arn:aws:iam::${ACCOUNT_ID}:role/${ROLE_NAME}"

echo "▶ S3-бакети"
for b in "$COVERS_BUCKET" "$THUMBS_BUCKET"; do
  awsf s3api head-bucket --bucket "$b" 2>/dev/null || awsf s3api create-bucket --bucket "$b" >/dev/null
  echo "  ✓ $b"
done

echo "▶ IAM-роль (мінімальні права — слайди 32–33)"
cat > /tmp/trust.json <<JSON
{"Version":"2012-10-17","Statement":[{"Effect":"Allow",
 "Principal":{"Service":"lambda.amazonaws.com"},"Action":"sts:AssumeRole"}]}
JSON
awsf iam get-role --role-name "$ROLE_NAME" >/dev/null 2>&1 \
  || awsf iam create-role --role-name "$ROLE_NAME" \
       --assume-role-policy-document file:///tmp/trust.json >/dev/null

cat > /tmp/policy.json <<JSON
{"Version":"2012-10-17","Statement":[
 {"Sid":"Logs","Effect":"Allow",
  "Action":["logs:CreateLogGroup","logs:CreateLogStream","logs:PutLogEvents"],
  "Resource":"arn:aws:logs:*:*:log-group:/aws/lambda/libratrack-*:*"},
 {"Sid":"ReadCovers","Effect":"Allow","Action":"s3:GetObject",
  "Resource":"arn:aws:s3:::${COVERS_BUCKET}/*"},
 {"Sid":"WriteThumbs","Effect":"Allow","Action":"s3:PutObject",
  "Resource":"arn:aws:s3:::${THUMBS_BUCKET}/*"}
]}
JSON
awsf iam put-role-policy --role-name "$ROLE_NAME" \
  --policy-name libratrack-minimal --policy-document file:///tmp/policy.json
echo "  ✓ $ROLE_ARN"

upsert_function() {  # <name> <zip> <timeout> <memory> [env-json]
  local name=$1 zip=$2 timeout=$3 mem=$4 env=${5:-}
  if awsf lambda get-function --function-name "$name" >/dev/null 2>&1; then
    awsf lambda update-function-code --function-name "$name" \
      --zip-file "fileb://$zip" >/dev/null
    local args=(--function-name "$name" --timeout "$timeout" --memory-size "$mem")
    [[ -n "$env" ]] && args+=(--environment "$env")
    awsf lambda update-function-configuration "${args[@]}" >/dev/null
    echo "  ✓ $name (оновлено)"
  else
    local args=(--function-name "$name" --runtime provided.al2023
      --handler bootstrap --architectures "$LAMBDA_ARCH"
      --role "$ROLE_ARN" --zip-file "fileb://$zip"
      --timeout "$timeout" --memory-size "$mem")
    [[ -n "$env" ]] && args+=(--environment "$env")
    awsf lambda create-function "${args[@]}" >/dev/null
    echo "  ✓ $name (створено)"
  fi
}

echo "▶ Lambda-функції (provided.al2023, ${LAMBDA_ARCH})"
upsert_function "$WEBHOOK_FN" build/webhook.zip   10 256
upsert_function "$THUMB_FN"   build/thumbnail.zip 30 512 \
  "Variables={OUTPUT_BUCKET=${THUMBS_BUCKET}}"
upsert_function "$FINE_FN"    build/fine.zip      10 256

echo "▶ API Gateway (REST, Lambda proxy)"
API_ID=$(awsf apigateway get-rest-apis \
  --query "items[?name=='${API_NAME}'].id | [0]" --output text)
if [[ -z "$API_ID" || "$API_ID" == "None" ]]; then
  API_ID=$(awsf apigateway create-rest-api --name "$API_NAME" --query id --output text)
fi
ROOT_ID=$(awsf apigateway get-resources --rest-api-id "$API_ID" \
  --query "items[?path=='/'].id | [0]" --output text)

# Додає POST /<path> з інтеграцією AWS_PROXY до Lambda <fn>; ідемпотентно.
add_route() {  # <path-part> <функція>
  local part=$1 fn=$2 res
  res=$(awsf apigateway get-resources --rest-api-id "$API_ID" \
    --query "items[?path=='/${part}'].id | [0]" --output text)
  if [[ -n "$res" && "$res" != "None" ]]; then echo "  ✓ /${part} (вже є)"; return; fi
  res=$(awsf apigateway create-resource --rest-api-id "$API_ID" \
    --parent-id "$ROOT_ID" --path-part "$part" --query id --output text)
  awsf apigateway put-method --rest-api-id "$API_ID" --resource-id "$res" \
    --http-method POST --authorization-type NONE >/dev/null
  awsf apigateway put-integration --rest-api-id "$API_ID" --resource-id "$res" \
    --http-method POST --type AWS_PROXY --integration-http-method POST \
    --uri "arn:aws:apigateway:${AWS_DEFAULT_REGION}:lambda:path/2015-03-31/functions/arn:aws:lambda:${AWS_DEFAULT_REGION}:${ACCOUNT_ID}:function:${fn}/invocations" \
    >/dev/null
  echo "  ✓ POST /${part} → ${fn}"
}
add_route webhook "$WEBHOOK_FN"
add_route fine "$FINE_FN"
awsf apigateway create-deployment --rest-api-id "$API_ID" --stage-name "$STAGE" >/dev/null
echo "$API_ID" > .api-id
echo "  ✓ API id: $API_ID"

echo "▶ Тригер S3 → Lambda (best effort)"
awsf lambda add-permission --function-name "$THUMB_FN" --statement-id s3-invoke \
  --action lambda:InvokeFunction --principal s3.amazonaws.com \
  --source-arn "arn:aws:s3:::${COVERS_BUCKET}" >/dev/null 2>&1 || true
THUMB_ARN="arn:aws:lambda:${AWS_DEFAULT_REGION}:${ACCOUNT_ID}:function:${THUMB_FN}"
if awsf s3api put-bucket-notification-configuration --bucket "$COVERS_BUCKET" \
     --notification-configuration "{\"LambdaFunctionConfigurations\":[{\"LambdaFunctionArn\":\"${THUMB_ARN}\",\"Events\":[\"s3:ObjectCreated:*\"]}]}" \
     >/dev/null 2>&1; then
  echo "  ✓ нотифікацію налаштовано"
else
  echo "  ⚠ Floci не прийняв нотифікацію — це не критично: smoke-тест викличе функцію вручну"
fi

echo
echo "✅ Готово. Webhook:"
echo "   curl -X POST ${AWS_ENDPOINT_URL}/restapis/${API_ID}/${STAGE}/_user_request_/webhook -d @events/payload_valid.json"
echo "   curl -X POST ${AWS_ENDPOINT_URL}/restapis/${API_ID}/${STAGE}/_user_request_/fine -d @events/fine_payload_valid.json"
