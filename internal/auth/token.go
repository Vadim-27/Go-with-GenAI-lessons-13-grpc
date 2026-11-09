package auth

import (
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// TokenTTL — час життя токена (claim exp).
const TokenTTL = 24 * time.Hour

// minSecretLen — мінімальна довжина секрету. Для HS256 потрібно щонайменше
// 256 біт (32 байти), інакше підпис легко підібрати.
const minSecretLen = 32

// ErrInvalidToken повертається, якщо токен зіпсований, підроблений або прострочений.
var ErrInvalidToken = errors.New("invalid or expired token")

// Claims — вміст JWT: id користувача, його роль і стандартні поля (exp, iat).
type Claims struct {
	UserID uint   `json:"user_id"`
	Role   string `json:"role"`
	jwt.RegisteredClaims
}

// secret читає ключ підпису ВИКЛЮЧНО зі змінної середовища JWT_SECRET.
func secret() ([]byte, error) {
	s := os.Getenv("JWT_SECRET")
	if len(s) < minSecretLen {
		return nil, fmt.Errorf("JWT_SECRET must be set and at least %d characters long", minSecretLen)
	}
	return []byte(s), nil
}

// ValidateSecret перевіряє секрет під час старту: застосунок не повинен
// запускатися з порожнім або слабким ключем.
func ValidateSecret() error {
	_, err := secret()
	return err
}

// GenerateToken створює підписаний JWT (HS256) для користувача на TokenTTL.
func GenerateToken(userID uint, role string) (string, error) {
	key, err := secret()
	if err != nil {
		return "", err
	}
	now := time.Now()
	claims := Claims{
		UserID: userID,
		Role:   role,
		RegisteredClaims: jwt.RegisteredClaims{
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(TokenTTL)),
		},
	}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(key)
}

// ParseToken перевіряє підпис і термін дії токена та повертає його claims.
// Приймається лише HS256: це блокує атаку підміни алгоритму (alg=none).
func ParseToken(tokenString string) (*Claims, error) {
	key, err := secret()
	if err != nil {
		return nil, err
	}
	claims := &Claims{}
	token, err := jwt.ParseWithClaims(tokenString, claims,
		func(*jwt.Token) (any, error) { return key, nil },
		jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}),
		jwt.WithExpirationRequired(), // токен без exp вважається недійсним
	)
	if err != nil || !token.Valid {
		return nil, ErrInvalidToken
	}
	return claims, nil
}
