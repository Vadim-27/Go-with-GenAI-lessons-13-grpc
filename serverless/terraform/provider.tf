# Провайдер спрямовано на Floci (http://localhost:4566).
# Ендпоїнт треба вказати для КОЖНОГО сервісу, який використовує конфігурація.
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
    s3             = var.floci_endpoint
    sts            = var.floci_endpoint
  }
}
