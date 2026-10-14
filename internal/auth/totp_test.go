// SPDX-License-Identifier: AGPL-3.0-or-later

package auth

import (
	"testing"
	"time"

	"github.com/pquerna/otp/totp"
)

// TestTOTPCounterReplay проверяет, что counter одного и того же
// кода не меняется в пределах 30-секундного окна — это основа
// replay-защиты (P1-8).
func TestTOTPCounterReplay(t *testing.T) {
	secret, _, err := GenerateTOTPSecret("test")
	if err != nil {
		t.Fatal(err)
	}

	// Код для текущего шага.
	code, err := totp.GenerateCode(secret, time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}

	// Первый раз должен подойти.
	c1, ok := totpCounter(secret, code, time.Now().UTC())
	if !ok {
		t.Fatal("first code should be valid")
	}

	// Тот же код через 1 секунду — counter тот же.
	c2, ok := totpCounter(secret, code, time.Now().Add(1*time.Second).UTC())
	if !ok {
		t.Fatal("code should still be valid within window")
	}
	if c1 != c2 {
		t.Fatalf("counter should match, got %d vs %d", c1, c2)
	}
}

// TestTOTPCounterRejectsGarbage проверяет, что мусорные коды
// не проходят.
func TestTOTPCounterRejectsGarbage(t *testing.T) {
	secret, _, err := GenerateTOTPSecret("test")
	if err != nil {
		t.Fatal(err)
	}

	for _, bad := range []string{"", "12345", "abcdef", "00000000"} {
		if _, ok := totpCounter(secret, bad, time.Now().UTC()); ok {
			t.Errorf("code %q should be rejected", bad)
		}
	}
}
