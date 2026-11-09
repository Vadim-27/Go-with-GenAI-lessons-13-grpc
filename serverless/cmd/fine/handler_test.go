package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/aws/aws-lambda-go/events"
)

func call(t *testing.T, req events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, map[string]any) {
	t.Helper()
	resp, err := handler(context.Background(), req)
	if err != nil {
		t.Fatalf("handler must not return a Go error for client mistakes (would be 502): %v", err)
	}
	var out map[string]any
	if err := json.Unmarshal([]byte(resp.Body), &out); err != nil {
		t.Fatalf("response body is not JSON: %v (%q)", err, resp.Body)
	}
	return resp, out
}

func TestCalculate(t *testing.T) {
	cases := []struct {
		name        string
		category    string
		due, ret    string
		wantDays    int
		wantKopecks int
		wantCapped  bool
		wantFmt     string
	}{
		{"on time", "standard", "2026-10-10", "2026-10-10", 0, 0, false, "0.00 UAH"},
		{"early", "standard", "2026-10-10", "2026-10-05", 0, 0, false, "0.00 UAH"},
		{"standard 9 days", "standard", "2026-10-01", "2026-10-10", 9, 1800, false, "18.00 UAH"},
		{"new release 3 days", "new_release", "2026-10-01", "2026-10-04", 3, 1500, false, "15.00 UAH"},
		{"category is case-insensitive", " Reference ", "2026-10-01", "2026-10-02", 1, 1000, false, "10.00 UAH"},
		{"capped", "standard", "2026-01-01", "2026-10-01", 273, 20000, true, "200.00 UAH"},
		{"leap year boundary", "standard", "2028-02-28", "2028-03-01", 2, 400, false, "4.00 UAH"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			res, problems := calculate(LoanReturn{LoanID: "L-1", Category: c.category, DueDate: c.due, ReturnedDate: c.ret})
			if len(problems) > 0 {
				t.Fatalf("unexpected problems: %v", problems)
			}
			if res.DaysOverdue != c.wantDays || res.FineKopecks != c.wantKopecks ||
				res.Capped != c.wantCapped || res.FineFormatted != c.wantFmt {
				t.Errorf("got %+v", res)
			}
		})
	}
}

func TestValidPayload(t *testing.T) {
	resp, out := call(t, events.APIGatewayProxyRequest{
		Body: `{"loan_id":"L-42","category":"STANDARD","due_date":"2026-10-01","returned_date":"2026-10-10"}`,
	})
	if resp.StatusCode != 200 {
		t.Fatalf("status = %d, want 200 (%s)", resp.StatusCode, resp.Body)
	}
	r := out["result"].(map[string]any)
	if r["fine"] != "18.00 UAH" || r["category"] != "standard" {
		t.Errorf("unexpected result: %v", r)
	}
}

func TestBase64Body(t *testing.T) {
	raw := `{"loan_id":"L-1","category":"standard","due_date":"2026-10-01","returned_date":"2026-10-02"}`
	resp, _ := call(t, events.APIGatewayProxyRequest{
		Body:            base64.StdEncoding.EncodeToString([]byte(raw)),
		IsBase64Encoded: true,
	})
	if resp.StatusCode != 200 {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
}

func TestClientErrorsAre400NotPanicNot502(t *testing.T) {
	cases := map[string]events.APIGatewayProxyRequest{
		"malformed json":   {Body: `{"loan_id":`},
		"empty body":       {Body: ``},
		"bad base64":       {Body: "%%%", IsBase64Encoded: true},
		"json array":       {Body: `[1,2,3]`},
		"empty object":     {Body: `{}`},
		"unknown category": {Body: `{"loan_id":"L","category":"vip","due_date":"2026-10-01","returned_date":"2026-10-02"}`},
		"bad date format":  {Body: `{"loan_id":"L","category":"standard","due_date":"01.10.2026","returned_date":"2026-10-02"}`},
		"impossible date":  {Body: `{"loan_id":"L","category":"standard","due_date":"2026-02-30","returned_date":"2026-03-02"}`},
		"wrong type":       {Body: `{"loan_id":123}`},
	}
	for name, req := range cases {
		t.Run(name, func(t *testing.T) {
			resp, out := call(t, req)
			if resp.StatusCode != 400 {
				t.Fatalf("status = %d, want 400 (%s)", resp.StatusCode, resp.Body)
			}
			if out["error"] == nil {
				t.Errorf("response must carry an error message: %s", resp.Body)
			}
		})
	}
}

func TestAllValidationProblemsAreReported(t *testing.T) {
	_, out := call(t, events.APIGatewayProxyRequest{Body: `{}`})
	if details := out["details"].([]any); len(details) != 4 {
		t.Fatalf("expected 4 problems at once, got %v", details)
	}
}

// Фікстури з events/ — і вхід для `sam local invoke` / `aws lambda invoke`,
// і регресійні тести (слайд 20).
func TestEventFixtures(t *testing.T) {
	want := map[string]int{
		"fine_valid.json":         200,
		"fine_capped.json":        200,
		"fine_invalid.json":       400,
		"fine_missing_field.json": 400,
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
