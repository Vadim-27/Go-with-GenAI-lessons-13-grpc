package model_test

import (
	"encoding/json"
	"testing"

	"libratrack/internal/model"
)

func TestNewErrorResponse_JSON(t *testing.T) {
	tests := []struct {
		name    string
		code    string
		message string
		want    string
	}{
		{"not found", model.CodeNotFound, "Book not found",
			`{"error":{"code":"not_found","message":"Book not found"}}`},
		{"validation", model.CodeValidationError, "Invalid request body",
			`{"error":{"code":"validation_error","message":"Invalid request body"}}`},
		{"internal", model.CodeInternalServerError, "Internal server error",
			`{"error":{"code":"internal_server_error","message":"Internal server error"}}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			raw, err := json.Marshal(model.NewErrorResponse(tt.code, tt.message))
			if err != nil {
				t.Fatal(err)
			}
			if string(raw) != tt.want {
				t.Errorf("json = %s, want %s", raw, tt.want)
			}
		})
	}
}
