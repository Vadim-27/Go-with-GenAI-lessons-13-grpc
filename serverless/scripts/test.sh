#!/usr/bin/env bash
# Smoke-тест на Floci за чеклістом зі слайда 45.
# Використання: scripts/test.sh
set -uo pipefail
cd "$(dirname "$0")/.."
source scripts/env.sh

PASS=0; FAIL=0
ok()   { echo "  ✅ $1"; PASS=$((PASS+1)); }
bad()  { echo "  ❌ $1"; FAIL=$((FAIL+1)); }
check() { # <опис> <очікуване> <фактичне>
  if [[ "$2" == "$3" ]]; then ok "$1 (=$3)"; else bad "$1: очікував $2, отримав '$3'"; fi
}

API_ID="${API_ID:-$(cat .api-id 2>/dev/null || true)}"
if [[ -z "$API_ID" ]]; then
  API_ID=$(awsf apigateway get-rest-apis --query "items[?name=='${API_NAME}'].id | [0]" --output text 2>/dev/null)
fi
[[ -n "$API_ID" && "$API_ID" != "None" ]] || { echo "✗ API не знайдено. Спершу: make deploy-cli  (або make deploy-tf)"; exit 1; }
URL="${AWS_ENDPOINT_URL}/restapis/${API_ID}/${STAGE}/_user_request_/webhook"
echo "API: $URL"

invoke_status() { # <функція> <файл події> -> statusCode із відповіді Lambda proxy
  local out; out=$(mktemp)
  awsf lambda invoke --function-name "$1" --payload "fileb://$2" \
    --cli-binary-format raw-in-base64-out "$out" >/dev/null 2>&1
  grep -o '"statusCode":[0-9]*' "$out" | head -1 | cut -d: -f2
  rm -f "$out"
}
http_status() { # <файл тіла> -> HTTP-код від API Gateway
  curl -s -o /dev/null -w '%{http_code}' -X POST -H 'Content-Type: application/json' \
    --data-binary "@$1" "$URL"
}

echo; echo "── 1. Пряме викликання Lambda (aws lambda invoke) ──"
check "valid.json → 200"             200 "$(invoke_status "$WEBHOOK_FN" events/valid.json)"
check "invalid.json → 400, не 502"   400 "$(invoke_status "$WEBHOOK_FN" events/invalid.json)"
check "missing_field.json → 400"     400 "$(invoke_status "$WEBHOOK_FN" events/missing_field.json)"
check "unsupported_event.json → 400" 400 "$(invoke_status "$WEBHOOK_FN" events/unsupported_event.json)"

echo; echo "── 2. Через API Gateway (curl) ──"
check "валідний payload → 200"        200 "$(http_status events/payload_valid.json)"
check "зламаний JSON → 400 (не 502)"  400 "$(http_status events/payload_malformed.json)"
check "бракує поля → 400"             400 "$(http_status events/payload_missing_field.json)"
echo "  Відповідь на валідний запит:"
curl -s -X POST -H 'Content-Type: application/json' --data-binary @events/payload_valid.json "$URL" \
  | sed 's/^/    /'; echo

FINE_URL="${AWS_ENDPOINT_URL}/restapis/${API_ID}/${STAGE}/_user_request_/fine"
fine_http_status() { curl -s -o /dev/null -w '%{http_code}' -X POST -H 'Content-Type: application/json' --data-binary "@$1" "$FINE_URL"; }
echo; echo "── 2b. fine (домашнє завдання) ──"
check "fine: пряме викликання fine_valid.json → 200"   200 "$(invoke_status "$FINE_FN" events/fine_valid.json)"
check "fine: fine_invalid.json → 400"                  400 "$(invoke_status "$FINE_FN" events/fine_invalid.json)"
check "fine: API, валідний payload → 200"              200 "$(fine_http_status events/fine_payload_valid.json)"
check "fine: API, зламаний JSON → 400 (не 502)"        400 "$(fine_http_status events/fine_payload_malformed.json)"
check "fine: API, бракує полів → 400"                  400 "$(fine_http_status events/fine_payload_missing_field.json)"

echo; echo "── 3. S3-тригер: мініатюра обкладинки ──"
awsf s3 rm "s3://${THUMBS_BUCKET}/thumbs/covers/cover.png" >/dev/null 2>&1 || true
awsf s3 cp testdata/cover.png "s3://${COVERS_BUCKET}/covers/cover.png" >/dev/null
found=0
for _ in $(seq 1 15); do
  if awsf s3api head-object --bucket "$THUMBS_BUCKET" --key thumbs/covers/cover.png >/dev/null 2>&1; then
    found=1; break
  fi
  sleep 1
done
if [[ $found == 1 ]]; then
  ok "мініатюра створена автоматично (S3-тригер спрацював)"
else
  echo "  ℹ️  Автоматичний тригер не спрацював — викликаю функцію вручну із events/s3_put.json"
  awsf lambda invoke --function-name "$THUMB_FN" --payload fileb://events/s3_put.json \
    --cli-binary-format raw-in-base64-out /tmp/thumb-out.json >/dev/null 2>&1
  if awsf s3api head-object --bucket "$THUMBS_BUCKET" --key thumbs/covers/cover.png >/dev/null 2>&1; then
    ok "мініатюра створена (ручний виклик)"
  else
    bad "мініатюру не створено. Дивіться: docker compose logs floci | tail -50"
  fi
fi
if awsf s3 cp "s3://${THUMBS_BUCKET}/thumbs/covers/cover.png" /tmp/thumb.png >/dev/null 2>&1; then
  command -v file >/dev/null && echo "    $(file /tmp/thumb.png | cut -d: -f2)"
  ok "мініатюру можна завантажити ($(wc -c < /tmp/thumb.png | tr -d ' ') байт)"
fi

echo; echo "── Підсумок: ✅ ${PASS}  ❌ ${FAIL} ──"
[[ $FAIL -eq 0 ]]
