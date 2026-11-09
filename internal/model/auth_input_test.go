package model

import (
	"errors"
	"strings"
	"testing"
)

func ptr(s string) *string { return &s }

func TestRegisterRequest_Validate(t *testing.T) {
	tests := []struct {
		name    string
		in      RegisterRequest
		wantErr bool
	}{
		{"коректний", RegisterRequest{Email: ptr("user@libratrack.com"), Password: ptr("SecurePass123!")}, false},
		{"без email", RegisterRequest{Password: ptr("SecurePass123!")}, true},
		{"поганий email", RegisterRequest{Email: ptr("not-an-email"), Password: ptr("SecurePass123!")}, true},
		{"email з іменем", RegisterRequest{Email: ptr("Bob <bob@x.com>"), Password: ptr("SecurePass123!")}, true},
		{"без пароля", RegisterRequest{Email: ptr("user@libratrack.com")}, true},
		{"короткий пароль", RegisterRequest{Email: ptr("user@libratrack.com"), Password: ptr("short")}, true},
		{"пароль довший за 72 байти", RegisterRequest{Email: ptr("user@libratrack.com"), Password: ptr(strings.Repeat("a", 73))}, true},
		{"пароль рівно 72 байти", RegisterRequest{Email: ptr("user@libratrack.com"), Password: ptr(strings.Repeat("a", 72))}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.in.Validate()
			if (err != nil) != tt.wantErr {
				t.Fatalf("Validate() = %v, wantErr %v", err, tt.wantErr)
			}
			if err != nil && !errors.Is(err, ErrValidation) {
				t.Errorf("помилка має загортати ErrValidation: %v", err)
			}
		})
	}
}

func TestRegisterRequest_ToUser_AlwaysReader(t *testing.T) {
	in := RegisterRequest{Email: ptr("  User@LibraTrack.com "), Password: ptr("SecurePass123!")}
	u := in.ToUser("hash")
	if u.Role != RoleReader {
		t.Errorf("role = %q, потрібно reader", u.Role)
	}
	if u.Email != "user@libratrack.com" {
		t.Errorf("email не нормалізовано: %q", u.Email)
	}
	if u.PasswordHash != "hash" {
		t.Errorf("password_hash = %q", u.PasswordHash)
	}
}

func TestLoginRequest_Validate(t *testing.T) {
	if err := (LoginRequest{Email: ptr("a@b.c"), Password: ptr("x")}).Validate(); err != nil {
		t.Errorf("очікували успіх, отримали %v", err)
	}
	if err := (LoginRequest{Email: ptr("a@b.c")}).Validate(); err == nil {
		t.Error("без пароля має бути помилка")
	}
}
