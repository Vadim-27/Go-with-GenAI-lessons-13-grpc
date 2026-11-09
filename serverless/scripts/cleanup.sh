#!/usr/bin/env bash
# Видаляє всі ресурси демо з Floci (ідемпотентно).
set -uo pipefail
cd "$(dirname "$0")/.."
source scripts/env.sh

echo "▶ API Gateway"
for id in $(awsf apigateway get-rest-apis --query "items[?name=='${API_NAME}'].id" --output text 2>/dev/null); do
  awsf apigateway delete-rest-api --rest-api-id "$id" >/dev/null 2>&1 && echo "  ✓ $id"
done
rm -f .api-id

echo "▶ Lambda"
for fn in "$WEBHOOK_FN" "$THUMB_FN" "$FINE_FN"; do
  awsf lambda delete-function --function-name "$fn" >/dev/null 2>&1 && echo "  ✓ $fn"
done

echo "▶ S3"
for b in "$COVERS_BUCKET" "$THUMBS_BUCKET"; do
  awsf s3 rm "s3://$b" --recursive >/dev/null 2>&1
  awsf s3api delete-bucket --bucket "$b" >/dev/null 2>&1 && echo "  ✓ $b"
done

echo "▶ IAM"
awsf iam delete-role-policy --role-name "$ROLE_NAME" --policy-name libratrack-minimal >/dev/null 2>&1
awsf iam delete-role --role-name "$ROLE_NAME" >/dev/null 2>&1 && echo "  ✓ $ROLE_NAME"
echo "✅ Очищено"
