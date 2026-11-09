package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"strings"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambdacontext"
)

// WebhookPayload — зовнішня «обгортка» події. json.RawMessage відкладає розбір
// поля data, бо його структура залежить від типу події (слайд 19).
type WebhookPayload struct {
	Event string          `json:"event"`
	Data  json.RawMessage `json:"data"`
}

const supportedEvent = "book.donated"

// handler — HTTP-тригер через API Gateway (REST, Lambda proxy).
//
// Важливо (слайд 19): очікувані помилки клієнта (4xx) повертаємо як ЗВИЧАЙНУ
// відповідь і nil-error. Повернення non-nil error дало б 502 від API Gateway.
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

	var payload WebhookPayload
	if err := json.Unmarshal([]byte(body), &payload); err != nil {
		log.WarnContext(ctx, "invalid json", "error", err)
		return respond(requestID, 400, map[string]any{"error": "invalid json"}), nil
	}

	if payload.Event != supportedEvent {
		log.WarnContext(ctx, "unsupported event", "event", payload.Event)
		return respond(requestID, 400, map[string]any{
			"error": "unsupported event",
			"event": payload.Event,
		}), nil
	}

	var in Donation
	if len(payload.Data) == 0 || json.Unmarshal(payload.Data, &in) != nil {
		return respond(requestID, 400, map[string]any{"error": "data must be a JSON object"}), nil
	}

	donation, problems := normalize(in)
	if len(problems) > 0 {
		log.WarnContext(ctx, "validation failed", "problems", problems)
		return respond(requestID, 400, map[string]any{
			"error":   "validation failed",
			"details": problems,
		}), nil
	}

	log.InfoContext(ctx, "donation processed", "isbn", donation.ISBN)
	return respond(requestID, 200, map[string]any{
		"status":   "processed",
		"donation": donation,
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
