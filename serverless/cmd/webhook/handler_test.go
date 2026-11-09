package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"strings"
	"testing"

	"github.com/aws/aws-lambda-go/events"
)

const validBody = `{"event":"book.donated","data":{
  "title":"  The Go   Programming Language ",
  "author":"Donovan & Kernighan",
  "isbn":"978-0-13-419044-0",
  "condition":"GOOD",
  "donor_email":"Reader@Example.COM"}}`

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

func TestValidPayloadIsNormalized(t *testing.T) {
	resp, out := call(t, events.APIGatewayProxyRequest{Body: validBody})
	if resp.StatusCode != 200 {
		t.Fatalf("status = %d, want 200 (%s)", resp.StatusCode, resp.Body)
	}
	d := out["donation"].(map[string]any)
	want := map[string]string{
		"title":       "The Go Programming Language",
		"isbn":        "9780134190440",
		"condition":   "good",
		"donor_email": "reader@example.com",
	}
	for k, v := range want {
		if d[k] != v {
			t.Errorf("%s = %v, want %v", k, d[k], v)
		}
	}
}

func TestBase64Body(t *testing.T) {
	resp, _ := call(t, events.APIGatewayProxyRequest{
		Body:            base64.StdEncoding.EncodeToString([]byte(validBody)),
		IsBase64Encoded: true,
	})
	if resp.StatusCode != 200 {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
}

func TestClientErrorsAre400NotPanicNot502(t *testing.T) {
	cases := map[string]string{
		"malformed json":    `{"event":`,
		"empty body":        ``,
		"unsupported event": `{"event":"book.lost","data":{}}`,
		"data not object":   `{"event":"book.donated","data":"oops"}`,
		"missing data":      `{"event":"book.donated"}`,
		"missing fields":    `{"event":"book.donated","data":{}}`,
		"bad isbn checksum": `{"event":"book.donated","data":{"title":"T","author":"A","isbn":"9780134190441","condition":"new","donor_email":"a@b.co"}}`,
		"bad condition":     `{"event":"book.donated","data":{"title":"T","author":"A","isbn":"9780134190440","condition":"shiny","donor_email":"a@b.co"}}`,
		"bad email":         `{"event":"book.donated","data":{"title":"T","author":"A","isbn":"9780134190440","condition":"new","donor_email":"nope"}}`,
	}
	for name, body := range cases {
		t.Run(name, func(t *testing.T) {
			resp, out := call(t, events.APIGatewayProxyRequest{Body: body})
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
	_, out := call(t, events.APIGatewayProxyRequest{Body: `{"event":"book.donated","data":{}}`})
	details := out["details"].([]any)
	if len(details) < 4 {
		t.Fatalf("expected several problems at once, got %v", details)
	}
}

func TestISBN(t *testing.T) {
	good := []string{"0-306-40615-2", "9780134190440", "097522980X", "978 0 13 419044 0"}
	for _, s := range good {
		if _, err := normalizeISBN(s); err != nil {
			t.Errorf("%q should be valid: %v", s, err)
		}
	}
	bad := []string{"", "123", "0306406153", "978013419044X", strings.Repeat("9", 13)}
	for _, s := range bad {
		if _, err := normalizeISBN(s); err == nil {
			t.Errorf("%q should be invalid", s)
		}
	}
}
