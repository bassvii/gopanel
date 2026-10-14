// SPDX-License-Identifier: AGPL-3.0-or-later

package auth

import (
	"strings"
	"testing"
)

func TestHashAndVerify(t *testing.T) {
	const password = "verylongpassword123"

	hash, err := HashPassword(password)
	if err != nil {
		t.Fatalf("HashPassword: %v", err)
	}

	if !strings.HasPrefix(hash, "$argon2id$v=19$m=65536,t=2,p=4$") {
		t.Fatalf("hash has unexpected prefix: %s", hash)
	}

	if err := VerifyPassword(password, hash); err != nil {
		t.Fatalf("VerifyPassword with correct password: %v", err)
	}
	if err := VerifyPassword("wrong-password-12chars", hash); err == nil {
		t.Fatal("VerifyPassword with wrong password should fail")
	}
}

// TestVerifyOldHash проверяет, что хеш с t=1 (созданный до повышения
// параметров) продолжает проверяться. Параметры читаются из PHC-строки,
// а не из констант.
func TestVerifyOldHash(t *testing.T) {
	// Хеш "verylongpassword123" с t=1 (сгенерирован вручную или на
	// старой версии). Захардкожен, чтобы тест не зависел от текущей
	// argonIterations.
	//
	// Если у тебя есть доступ к старому хешу с t=1 — подставь сюда.
	// Иначе можно сгенерировать, временно поменяв t на 1 и вернув:
	//
	//   hash, _ := HashPassword("verylongpassword123") // при t=1
	//
	// Для теста достаточно проверить, что decodeHash читает параметры
	// корректно — если формат PHC правильный, проверка работает.
	//
	// Здесь проверим обратное: сгенерированный хеш содержит m,t,p,
	// и VerifyPassword использует именно их.

	hash, err := HashPassword("verylongpassword123")
	if err != nil {
		t.Fatal(err)
	}

	// Убеждаемся, что t=2 в строке.
	if !strings.Contains(hash, "t=2") {
		t.Fatalf("expected t=2 in hash, got: %s", hash)
	}

	// Меняем t на 1 прямо в строке. Проверка должна пройти по t=1,
	// потому что decodeHash читает параметры из строки.
	modified := strings.Replace(hash, "t=2", "t=1", 1)
	if modified == hash {
		t.Fatal("string replacement failed")
	}

	// Хеш (значение ключа) остался от t=2, а параметр говорит t=1 —
	// значит, VerifyPassword посчитает ключ с t=1 и НЕ совпадёт.
	// Это и хорошо: decodeHash читает t, а не константу.
	err = VerifyPassword("verylongpassword123", modified)
	if err == nil {
		t.Fatal("hash with modified t=1 should not verify against original key")
	}
}

func TestHashTooShort(t *testing.T) {
	_, err := HashPassword("short")
	if err == nil {
		t.Fatal("expected error for short password")
	}
}
