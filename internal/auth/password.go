// Package auth містить хешування паролів (bcrypt) та видачу/перевірку JWT.
package auth

import "golang.org/x/crypto/bcrypt"

// HashPassword повертає bcrypt-хеш пароля з вартістю за замовчуванням (10).
// Сіль генерується bcrypt автоматично і зберігається всередині хеша.
func HashPassword(password string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(hash), nil
}

// CheckPassword повертає true, якщо пароль відповідає хешу.
// Порівняння виконується за сталий час, це захищає від timing-атак.
func CheckPassword(hash, password string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) == nil
}

// dummyHash — хеш, з яким порівнюємо, коли користувача з таким email немає.
// Так login витрачає однаковий час і для відомого, і для невідомого email,
// тож за часом відповіді не можна визначити, чи існує акаунт.
// Обчислюється один раз під час старту програми.
var dummyHash, _ = HashPassword("libratrack-dummy-password")

// CheckDummyPassword виконує «холосте» порівняння bcrypt (результат ігнорується).
func CheckDummyPassword(password string) {
	_ = CheckPassword(dummyHash, password)
}
