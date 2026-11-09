output "api_id" {
  value = aws_api_gateway_rest_api.api.id
}

output "webhook_url_floci" {
  description = "URL webhook у Floci (шлях _user_request_ працює без wildcard-DNS)."
  value       = "${var.floci_endpoint}/restapis/${aws_api_gateway_rest_api.api.id}/${aws_api_gateway_stage.stage.stage_name}/_user_request_/webhook"
}

output "try_it" {
  value = "curl -X POST ${var.floci_endpoint}/restapis/${aws_api_gateway_rest_api.api.id}/${aws_api_gateway_stage.stage.stage_name}/_user_request_/webhook -d @events/payload_valid.json"
}

output "fine_url_floci" {
  description = "URL fine-функції у Floci."
  value       = "${var.floci_endpoint}/restapis/${aws_api_gateway_rest_api.api.id}/${aws_api_gateway_stage.stage.stage_name}/_user_request_/fine"
}
