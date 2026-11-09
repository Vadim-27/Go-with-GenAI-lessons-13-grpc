#!/usr/bin/env bash
# Demo 1 (слайд 32): EC2 API у Floci. Запускати у VM: bash /app/demos/ec2/run.sh
# Емулятор зберігає лише метадані інстансу, реальна VM не запускається.
set -euo pipefail

# awslocal (awscli-local) застарів і друкує попередження на кожен виклик;
# це те саме, що aws з endpoint емулятора.
awslocal() { aws --endpoint-url=http://localhost:4566 "$@"; }

echo "==> Key pair demo"
if ! awslocal ec2 describe-key-pairs --key-names demo >/dev/null 2>&1; then
  awslocal ec2 create-key-pair --key-name demo --query 'KeyName' --output text
fi

echo "==> Доступні образи (перші 5)"
awslocal ec2 describe-images --query 'Images[:5].[ImageId,Name]' --output table
AMI=$(awslocal ec2 describe-images --query 'Images[0].ImageId' --output text)

echo "==> run-instances з $AMI"
awslocal ec2 run-instances --image-id "$AMI" --instance-type t3.micro --key-name demo \
  --tag-specifications 'ResourceType=instance,Tags=[{Key=Name,Value=demo-api}]' \
  --query 'Instances[0].[InstanceId,InstanceType,State.Name]' --output table

echo "==> describe-instances"
awslocal ec2 describe-instances \
  --query 'Reservations[].Instances[].[InstanceId,InstanceType,State.Name,Tags[?Key==`Name`]|[0].Value]' \
  --output table
