package model

import (
	"net/mail"
	"strings"
)

// Межі пароля. bcrypt обробляє максимум 72 БАЙТИ, решта мовчки відкидається,
// тому довші паролі забороняємо явно.
const (
	minPasswordLen = 8
	maxPasswordLen = 72
	maxEmailLen    = 254 // межа за RFC 5321
)

// RegisterRequest — тіло POST /auth/register.
//
// DTO навмисно містить ЛИШЕ поля, які вправі задати користувач. Немає id, role,
// password_hash, created_at: клієнт не може сам собі призначити роль admin
// (Mass Assignment). Роль завжди 'reader', її змінює тільки адміністратор.
type RegisterRequest struct {
	Email    *string `json:"email"`
	Password *string `json:"password"`
	FullName *string `json:"full_name"`
}

// Validate перевіряє email та пароль. Повертає помилку, що загортає ErrValidation.
func (in RegisterRequest) Validate() error {
	var problems []string
	if blank(in.Email) {
		problems = append(problems, "email is required")
	} else if !validEmail(normalizeEmail(*in.Email)) {
		problems = append(problems, "email is invalid")
	}
	if in.Password == nil || *in.Password == "" {
		problems = append(problems, "password is required")
	} else if n := len(*in.Password); n < minPasswordLen || n > maxPasswordLen {
		problems = append(problems, "password must be 8 to 72 bytes long")
	}
	if len(problems) > 0 {
		return invalid("%s", strings.Join(problems, "; "))
	}
	return nil
}

// ToUser створює користувача з роллю reader. PasswordHash заповнює хендлер
// після хешування пароля.
func (in RegisterRequest) ToUser(passwordHash string) User {
	u := User{
		Email:        normalizeEmail(*in.Email),
		PasswordHash: passwordHash,
		Role:         RoleReader,
	}
	if !blank(in.FullName) {
		name := strings.TrimSpace(*in.FullName)
		u.FullName = &name
	}
	return u
}

// LoginRequest — тіло POST /auth/login.
type LoginRequest struct {
	Email    *string `json:"email"`
	Password *string `json:"password"`
}

// Validate вимагає непорожні email та пароль. Формат не перевіряємо навмисно:
// будь-яка помилка далі дає однакову відповідь 401 "invalid credentials".
func (in LoginRequest) Validate() error {
	if blank(in.Email) || in.Password == nil || *in.Password == "" {
		return invalid("email and password are required")
	}
	return nil
}

// NormalizedEmail повертає email у нижньому регістрі без пробілів по краях.
func (in LoginRequest) NormalizedEmail() string { return normalizeEmail(*in.Email) }

func normalizeEmail(s string) string { return strings.ToLower(strings.TrimSpace(s)) }

// validEmail приймає лише «чистий» адрес виду user@host (без імені та кутових дужок).
func validEmail(s string) bool {
	if len(s) > maxEmailLen {
		return false
	}
	addr, err := mail.ParseAddress(s)
	return err == nil && addr.Address == s
}
