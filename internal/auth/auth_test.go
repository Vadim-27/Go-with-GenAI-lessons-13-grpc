package auth

import (
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

const testSecret = "unit-test-secret-at-least-32-characters-long"

func TestPasswordHashing(t *testing.T) {
	hash, err := HashPassword("SecurePassword123!")
	if err != nil {
		t.Fatal(err)
	}
	if hash == "SecurePassword123!" {
		t.Fatal("пароль не повинен зберігатися у відкритому вигляді")
	}
	if !CheckPassword(hash, "SecurePassword123!") {
		t.Error("правильний пароль має проходити перевірку")
	}
	if CheckPassword(hash, "wrong") {
		t.Error("неправильний пароль не повинен проходити перевірку")
	}
}

func TestTokenRoundTrip(t *testing.T) {
	t.Setenv("JWT_SECRET", testSecret)

	tok, err := GenerateToken(7, "librarian")
	if err != nil {
		t.Fatal(err)
	}
	claims, err := ParseToken(tok)
	if err != nil {
		t.Fatal(err)
	}
	if claims.UserID != 7 || claims.Role != "librarian" {
		t.Errorf("claims = %+v", claims)
	}
	if ttl := time.Until(claims.ExpiresAt.Time); ttl < 23*time.Hour || ttl > 24*time.Hour {
		t.Errorf("exp має бути ≈ +24 год, зараз %v", ttl)
	}
}

func TestParseTokenRejects(t *testing.T) {
	t.Setenv("JWT_SECRET", testSecret)

	sign := func(m jwt.SigningMethod, key any, exp time.Time) string {
		c := Claims{UserID: 1, Role: "admin", RegisteredClaims: jwt.RegisteredClaims{ExpiresAt: jwt.NewNumericDate(exp)}}
		s, err := jwt.NewWithClaims(m, c).SignedString(key)
		if err != nil {
			t.Fatal(err)
		}
		return s
	}
	future := time.Now().Add(time.Hour)

	tests := []struct {
		name  string
		token string
	}{
		{"прострочений", sign(jwt.SigningMethodHS256, []byte(testSecret), time.Now().Add(-time.Minute))},
		{"чужий секрет", sign(jwt.SigningMethodHS256, []byte("another-secret-that-is-long-enough-123"), future)},
		{"інший алгоритм HS512", sign(jwt.SigningMethodHS512, []byte(testSecret), future)},
		{"alg=none", sign(jwt.SigningMethodNone, jwt.UnsafeAllowNoneSignatureType, future)},
		{"сміття", "not-a-jwt"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := ParseToken(tt.token); err == nil {
				t.Error("токен мав бути відхилений")
			}
		})
	}
}

func TestSecretValidation(t *testing.T) {
	t.Setenv("JWT_SECRET", "")
	if ValidateSecret() == nil {
		t.Error("порожній секрет має бути відхилений")
	}
	t.Setenv("JWT_SECRET", "short")
	if _, err := GenerateToken(1, "reader"); err == nil {
		t.Error("короткий секрет має бути відхилений")
	}
}
