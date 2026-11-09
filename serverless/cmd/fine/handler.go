package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"strings"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambdacontext"
)

// handler — HTTP-тригер через API Gateway (REST, Lambda proxy).
//
// Очікувані помилки клієнта повертаємо як ЗВИЧАЙНУ відповідь 400 і nil-error:
// non-nil error дав би 502 від API Gateway (слайд 19).
func handler(ctx context.Context, req events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	requestID := requestIDFrom(ctx, req)
	log := logger.With("request_id", requestID)

	body := req.Body
	if req.IsBase64Encoded {
		decoded, err := base64.StdEncoding.DecodeString(body)
		if err != nil {
			log.WarnContext(ctx, "invalid base64 body", "error", err)
			return respond(requestID, 400, map[string]any{"error": "body is not valid base64"}), nil
		}
		body = string(decoded)
	}
	if strings.TrimSpace(body) == "" {
		return respond(requestID, 400, map[string]any{"error": "request body is empty"}), nil
	}

	var in LoanReturn
	if err := json.Unmarshal([]byte(body), &in); err != nil {
		log.WarnContext(ctx, "invalid json", "error", err)
		return respond(requestID, 400, map[string]any{"error": "invalid json"}), nil
	}

	result, problems := calculate(in)
	if len(problems) > 0 {
		log.WarnContext(ctx, "validation failed", "problems", problems)
		return respond(requestID, 400, map[string]any{
			"error":   "validation failed",
			"details": problems,
		}), nil
	}

	log.InfoContext(ctx, "fine calculated", "loan_id", result.LoanID, "fine_kopecks", result.FineKopecks)
	return respond(requestID, 200, map[string]any{
		"status": "calculated",
		"result": result,
	}), nil
}

func requestIDFrom(ctx context.Context, req events.APIGatewayProxyRequest) string {
	if lc, ok := lambdacontext.FromContext(ctx); ok && lc.AwsRequestID != "" {
		return lc.AwsRequestID
	}
	if req.RequestContext.RequestID != "" {
		return req.RequestContext.RequestID
	}
	return "local"
}

func respond(requestID string, status int, v any) events.APIGatewayProxyResponse {
	b, _ := json.Marshal(v)
	return events.APIGatewayProxyResponse{
		StatusCode: status,
		Headers: map[string]string{
			"Content-Type": "application/json",
			"X-Request-Id": requestID,
		},
		Body: string(b),
	}
}
