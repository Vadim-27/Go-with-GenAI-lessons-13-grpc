package model

import (
	"strings"
	"unicode/utf8"
)

const maxFullNameLen = 100

// ValidRole повідомляє, чи є role однією з відомих ролей.
func ValidRole(role string) bool {
	switch role {
	case RoleReader, RoleLibrarian, RoleAdmin:
		return true
	}
	return false
}

// UpdateProfileRequest — тіло PUT /me.
//
// Єдине поле, яке користувач може змінити сам, — full_name. Немає email, role,
// password_hash, id: інакше це було б Mass Assignment (самопризначення ролі).
type UpdateProfileRequest struct {
	FullName *string `json:"full_name"`
}

// Validate вимагає, щоб поле було передано (null/відсутнє — помилка).
// Порожній рядок дозволений і означає «очистити ім'я».
func (in UpdateProfileRequest) Validate() error {
	if in.FullName == nil {
		return invalid("full_name is required")
	}
	if utf8.RuneCountInString(strings.TrimSpace(*in.FullName)) > maxFullNameLen {
		return invalid("full_name must be at most %d characters long", maxFullNameLen)
	}
	return nil
}

// Value повертає ім'я без пробілів по краях; nil означає очищення (NULL у БД).
func (in UpdateProfileRequest) Value() *string {
	name := strings.TrimSpace(*in.FullName)
	if name == "" {
		return nil
	}
	return &name
}

// UpdateRoleRequest — тіло PUT /users/:id/role (лише admin).
type UpdateRoleRequest struct {
	Role *string `json:"role"`
}

// Validate вимагає одну з ролей: reader, librarian, admin.
func (in UpdateRoleRequest) Validate() error {
	if in.Role == nil || !ValidRole(*in.Role) {
		return invalid("role must be one of: %s, %s, %s", RoleReader, RoleLibrarian, RoleAdmin)
	}
	return nil
}
