package main

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/aws/aws-lambda-go/events"
)

// Фікстури з events/ — це і вхід для `sam local invoke` / `aws lambda invoke`,
// і регресійні тести (слайд 20: «events/ double as regression tests»).
func TestEventFixtures(t *testing.T) {
	want := map[string]int{
		"valid.json":             200,
		"invalid.json":           400,
		"missing_field.json":     400,
		"unsupported_event.json": 400,
	}
	for name, status := range want {
		t.Run(name, func(t *testing.T) {
			raw, err := os.ReadFile(filepath.Join("..", "..", "events", name))
			if err != nil {
				t.Fatal(err)
			}
			var req events.APIGatewayProxyRequest
			if err := json.Unmarshal(raw, &req); err != nil {
				t.Fatalf("fixture is not a valid API Gateway event: %v", err)
			}
			resp, err := handler(context.Background(), req)
			if err != nil {
				t.Fatalf("unexpected Go error: %v", err)
			}
			if resp.StatusCode != status {
				t.Fatalf("status = %d, want %d; body: %s", resp.StatusCode, status, resp.Body)
			}
		})
	}
}
