package repository_test

import (
	"bytes"
	"strings"
	"testing"

	"gorm.io/gorm"

	"libratrack/internal/model"
	"libratrack/internal/repository"
	"libratrack/internal/testutil"
)

// SQL-лог не повинен містити значень параметрів: email та хеш пароля
// не мають потрапляти в stdout/файли логів.
func TestSQLLogger_DoesNotLeakValues(t *testing.T) {
	db := testutil.NewDB(t, schemaName)

	var buf bytes.Buffer
	logged := db.Session(&gorm.Session{Logger: repository.SQLLogger(&buf)})

	const (
		email = "leak-check@libratrack.com"
		hash  = "$2a$10$secretHashValueThatMustNeverBeLogged"
	)
	users := repository.NewUserRepository(logged)
	if err := users.Create(ctx, &model.User{Email: email, PasswordHash: hash, Role: model.RoleReader}); err != nil {
		t.Fatal(err)
	}
	if _, err := users.FindByEmail(ctx, email); err != nil {
		t.Fatal(err)
	}

	out := buf.String()
	if !strings.Contains(out, "users") {
		t.Fatalf("очікували SQL у логах, отримали: %q", out)
	}
	for _, secret := range []string{email, hash} {
		if strings.Contains(out, secret) {
			t.Errorf("лог SQL містить значення %q:\n%s", secret, out)
		}
	}
}
