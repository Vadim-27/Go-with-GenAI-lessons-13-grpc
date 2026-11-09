# ╔══════════════════════════════════════════════════════════════════════════╗
# ║ ЧЕРНЕТКА, ЗГЕНЕРОВАНА ШІ (промпт 1 з docs/ai-prompts.md, адаптований під    ║
# ║ функцію fine). Файл збережено БЕЗ правок — це «до» рев'ю.                  ║
# ║ Аналіз рядок за рядком і висновки: HomeWork 12.md, розділ «Завдання 2».    ║
# ║ Фінальна версія після рев'ю — у ../terraform/main.tf (ресурси fine).       ║
# ╚══════════════════════════════════════════════════════════════════════════╝

terraform {
  required_version = ">= 1.5"
  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = "~> 6.0"
    }
  }
}

variable "region" {
  type    = string
  default = "us-east-1"
}

variable "function_name" {
  type    = string
  default = "libratrack-fine"
}

variable "floci_endpoint" {
  type    = string
  default = "http://localhost:4566"
}

provider "aws" {
  region     = var.region
  access_key = "test"
  secret_key = "test"

  skip_credentials_validation = true
  skip_metadata_api_check     = true
  skip_requesting_account_id  = true
  s3_use_path_style           = true

  endpoints {
    apigateway     = var.floci_endpoint
    cloudwatchlogs = var.floci_endpoint
    iam            = var.floci_endpoint
    lambda         = var.floci_endpoint
    sts            = var.floci_endpoint
  }
}

# ── IAM ─────────────────────────────────────────────────────────────────────
resource "aws_iam_role" "lambda_role" {
  name = "${var.function_name}-role"

  assume_role_policy = jsonencode({
    Version = "2012-10-17"
    Statement = [{
      Action    = "sts:AssumeRole"
      Effect    = "Allow"
      Principal = { Service = "lambda.amazonaws.com" }
    }]
  })
}

resource "aws_iam_role_policy" "lambda_logging" {
  name = "${var.function_name}-logging"
  role = aws_iam_role.lambda_role.id

  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [{
      Effect = "Allow"
      Action = [
        "logs:CreateLogGroup",
        "logs:CreateLogStream",
        "logs:PutLogEvents"
      ]
      Resource = "arn:aws:logs:*:*:*"
    }]
  })
}

# ── CloudWatch Logs ─────────────────────────────────────────────────────────
resource "aws_cloudwatch_log_group" "lambda" {
  name = "/aws/lambda/${var.function_name}"
}

# ── Lambda ──────────────────────────────────────────────────────────────────
resource "aws_lambda_function" "fine" {
  function_name = var.function_name
  role          = aws_iam_role.lambda_role.arn

  runtime       = "provided.al2023"
  architectures = ["x86_64"]
  handler       = "bootstrap"

  filename         = "../build/fine.zip"
  source_code_hash = filebase64sha256("../build/fine.zip")

  timeout     = 10
  memory_size = 256

  depends_on = [aws_cloudwatch_log_group.lambda]
}

# ── API Gateway (REST, AWS_PROXY) ───────────────────────────────────────────
resource "aws_api_gateway_rest_api" "api" {
  name = "${var.function_name}-api"
}

resource "aws_api_gateway_resource" "fine" {
  rest_api_id = aws_api_gateway_rest_api.api.id
  parent_id   = aws_api_gateway_rest_api.api.root_resource_id
  path_part   = "fine"
}

resource "aws_api_gateway_method" "post" {
  rest_api_id   = aws_api_gateway_rest_api.api.id
  resource_id   = aws_api_gateway_resource.fine.id
  http_method   = "POST"
  authorization = "NONE"
}

resource "aws_api_gateway_integration" "lambda" {
  rest_api_id             = aws_api_gateway_rest_api.api.id
  resource_id             = aws_api_gateway_resource.fine.id
  http_method             = aws_api_gateway_method.post.http_method
  integration_http_method = "POST"
  type                    = "AWS_PROXY"
  uri                     = aws_lambda_function.fine.invoke_arn
}

resource "aws_lambda_permission" "apigw" {
  statement_id  = "AllowAPIGatewayInvoke"
  action        = "lambda:InvokeFunction"
  function_name = aws_lambda_function.fine.function_name
  principal     = "apigateway.amazonaws.com"
  source_arn    = "${aws_api_gateway_rest_api.api.execution_arn}/*/*"
}

resource "aws_api_gateway_deployment" "api" {
  rest_api_id = aws_api_gateway_rest_api.api.id

  depends_on = [aws_api_gateway_integration.lambda]
}

resource "aws_api_gateway_stage" "dev" {
  rest_api_id   = aws_api_gateway_rest_api.api.id
  deployment_id = aws_api_gateway_deployment.api.id
  stage_name    = "dev"
}

output "invoke_url" {
  value = "${var.floci_endpoint}/restapis/${aws_api_gateway_rest_api.api.id}/${aws_api_gateway_stage.dev.stage_name}/_user_request_/fine"
}
