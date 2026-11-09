# ── Сховище ─────────────────────────────────────────────────────────────────
resource "aws_s3_bucket" "covers" {
  bucket        = var.covers_bucket
  force_destroy = true
}

resource "aws_s3_bucket" "thumbnails" {
  bucket        = var.thumbnails_bucket
  force_destroy = true
}

# ── IAM: мінімально необхідні права (слайди 32–33) ─────────────────────────
data "aws_iam_policy_document" "assume" {
  statement {
    actions = ["sts:AssumeRole"]
    principals {
      type        = "Service"
      identifiers = ["lambda.amazonaws.com"]
    }
  }
}

resource "aws_iam_role" "lambda_exec" {
  name               = "libratrack-lambda-role"
  assume_role_policy = data.aws_iam_policy_document.assume.json
}

# Жодних Action = "*". Кожен дозвіл — під конкретний ресурс.
data "aws_iam_policy_document" "lambda_permissions" {
  statement {
    sid       = "Logs"
    actions   = ["logs:CreateLogGroup", "logs:CreateLogStream", "logs:PutLogEvents"]
    resources = ["arn:aws:logs:*:*:log-group:/aws/lambda/libratrack-*:*"]
  }
  statement {
    sid       = "ReadCovers"
    actions   = ["s3:GetObject"]
    resources = ["${aws_s3_bucket.covers.arn}/*"]
  }
  statement {
    sid       = "WriteThumbnails"
    actions   = ["s3:PutObject"]
    resources = ["${aws_s3_bucket.thumbnails.arn}/*"]
  }
}

resource "aws_iam_role_policy" "lambda_permissions" {
  name   = "libratrack-minimal"
  role   = aws_iam_role.lambda_exec.id
  policy = data.aws_iam_policy_document.lambda_permissions.json
}

# ── Lambda ─────────────────────────────────────────────────────────────────
resource "aws_lambda_function" "webhook" {
  function_name = "libratrack-webhook"
  role          = aws_iam_role.lambda_exec.arn

  runtime       = "provided.al2023" # go1.x знято з підтримки (слайд 15)
  architectures = [var.lambda_architecture]
  handler       = "bootstrap"

  filename         = "${path.module}/../build/webhook.zip"
  source_code_hash = filebase64sha256("${path.module}/../build/webhook.zip")

  timeout     = 10 # р99 + запас, а не максимум (слайд 35)
  memory_size = 256
}

resource "aws_lambda_function" "thumbnail" {
  function_name = "libratrack-thumbnail"
  role          = aws_iam_role.lambda_exec.arn

  runtime       = "provided.al2023"
  architectures = [var.lambda_architecture]
  handler       = "bootstrap"

  filename         = "${path.module}/../build/thumbnail.zip"
  source_code_hash = filebase64sha256("${path.module}/../build/thumbnail.zip")

  timeout     = 30
  memory_size = 512 # обробка зображень упирається в CPU, а CPU росте разом із пам'яттю (слайд 36)

  environment {
    variables = {
      OUTPUT_BUCKET = aws_s3_bucket.thumbnails.bucket
    }
  }
}

# ── API Gateway (REST, Lambda proxy) ───────────────────────────────────────
resource "aws_api_gateway_rest_api" "api" {
  name = "libratrack-api"
}

resource "aws_api_gateway_resource" "webhook" {
  rest_api_id = aws_api_gateway_rest_api.api.id
  parent_id   = aws_api_gateway_rest_api.api.root_resource_id
  path_part   = "webhook"
}

resource "aws_api_gateway_method" "webhook_post" {
  rest_api_id   = aws_api_gateway_rest_api.api.id
  resource_id   = aws_api_gateway_resource.webhook.id
  http_method   = "POST"
  authorization = "NONE"
}

resource "aws_api_gateway_integration" "webhook" {
  rest_api_id             = aws_api_gateway_rest_api.api.id
  resource_id             = aws_api_gateway_resource.webhook.id
  http_method             = aws_api_gateway_method.webhook_post.http_method
  integration_http_method = "POST"
  type                    = "AWS_PROXY"
  uri                     = aws_lambda_function.webhook.invoke_arn
}

resource "aws_lambda_permission" "apigw_webhook" {
  statement_id  = "AllowAPIGatewayInvoke"
  action        = "lambda:InvokeFunction"
  function_name = aws_lambda_function.webhook.function_name
  principal     = "apigateway.amazonaws.com"
  source_arn    = "${aws_api_gateway_rest_api.api.execution_arn}/*/*"
}

resource "aws_api_gateway_deployment" "api" {
  rest_api_id = aws_api_gateway_rest_api.api.id

  triggers = {
    redeployment = sha1(jsonencode([
      aws_api_gateway_resource.webhook.id,
      aws_api_gateway_method.webhook_post.id,
      aws_api_gateway_integration.webhook.id,
      aws_api_gateway_resource.fine.id,
      aws_api_gateway_method.fine_post.id,
      aws_api_gateway_integration.fine.id,
    ]))
  }

  lifecycle {
    create_before_destroy = true
  }
}

resource "aws_api_gateway_stage" "stage" {
  rest_api_id   = aws_api_gateway_rest_api.api.id
  deployment_id = aws_api_gateway_deployment.api.id
  stage_name    = var.stage_name
}

# ── S3 → Lambda ─────────────────────────────────────────────────────────────
resource "aws_lambda_permission" "s3_thumbnail" {
  statement_id  = "AllowS3Invoke"
  action        = "lambda:InvokeFunction"
  function_name = aws_lambda_function.thumbnail.function_name
  principal     = "s3.amazonaws.com"
  source_arn    = aws_s3_bucket.covers.arn
}

resource "aws_s3_bucket_notification" "covers" {
  bucket = aws_s3_bucket.covers.id

  lambda_function {
    lambda_function_arn = aws_lambda_function.thumbnail.arn
    events              = ["s3:ObjectCreated:*"]
  }

  depends_on = [aws_lambda_permission.s3_thumbnail]
}

# ── Домашнє завдання: fine (штраф за прострочення) ──────────────────────────
# Окрема роль лише з логами (принцип найменших привілеїв): fine не чіпає S3,
# тому спільна роль lambda_exec з правами на бакети їй не підходить.
resource "aws_iam_role" "fine_exec" {
  name               = "libratrack-fine-role"
  assume_role_policy = data.aws_iam_policy_document.assume.json
}

data "aws_iam_policy_document" "fine_permissions" {
  statement {
    sid       = "Logs"
    actions   = ["logs:CreateLogGroup", "logs:CreateLogStream", "logs:PutLogEvents"]
    resources = ["arn:aws:logs:*:*:log-group:/aws/lambda/libratrack-fine:*"]
  }
}

resource "aws_iam_role_policy" "fine_permissions" {
  name   = "libratrack-fine-logs-only"
  role   = aws_iam_role.fine_exec.id
  policy = data.aws_iam_policy_document.fine_permissions.json
}

resource "aws_lambda_function" "fine" {
  function_name = "libratrack-fine"
  role          = aws_iam_role.fine_exec.arn

  runtime       = "provided.al2023"
  architectures = [var.lambda_architecture]
  handler       = "bootstrap"

  filename         = "${path.module}/../build/fine.zip"
  source_code_hash = filebase64sha256("${path.module}/../build/fine.zip")

  timeout     = 10 # арифметика над датами: p99 — одиниці мс, 10 с — із запасом
  memory_size = 128
}

resource "aws_api_gateway_resource" "fine" {
  rest_api_id = aws_api_gateway_rest_api.api.id
  parent_id   = aws_api_gateway_rest_api.api.root_resource_id
  path_part   = "fine"
}

resource "aws_api_gateway_method" "fine_post" {
  rest_api_id   = aws_api_gateway_rest_api.api.id
  resource_id   = aws_api_gateway_resource.fine.id
  http_method   = "POST"
  authorization = "NONE"
}

resource "aws_api_gateway_integration" "fine" {
  rest_api_id             = aws_api_gateway_rest_api.api.id
  resource_id             = aws_api_gateway_resource.fine.id
  http_method             = aws_api_gateway_method.fine_post.http_method
  integration_http_method = "POST"
  type                    = "AWS_PROXY"
  uri                     = aws_lambda_function.fine.invoke_arn
}

resource "aws_lambda_permission" "apigw_fine" {
  statement_id  = "AllowAPIGatewayInvokeFine"
  action        = "lambda:InvokeFunction"
  function_name = aws_lambda_function.fine.function_name
  principal     = "apigateway.amazonaws.com"
  source_arn    = "${aws_api_gateway_rest_api.api.execution_arn}/*/*"
}
