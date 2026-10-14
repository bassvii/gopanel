// SPDX-License-Identifier: AGPL-3.0-or-later

package auth

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
)

// EncPrefix — префикс для зашифрованных строк. Позволяет отличать
// их от открытого текста при миграции.
const EncPrefix = "enc:v1:"

// ErrNotEncrypted возвращается DecryptSecret, если строка не зашифрована.
var ErrNotEncrypted = errors.New("value is not encrypted")

// IsEncrypted сообщает, начинается ли строка с префикса EncPrefix.
func IsEncrypted(s string) bool {
	return strings.HasPrefix(s, EncPrefix)
}

// EncryptSecret шифрует plaintext алгоритмом AES-256-GCM.
// Формат вывода: "enc:v1:<base64(nonce || ciphertext)>".
// Для пустой строки возвращает пустую строку без изменений.
func EncryptSecret(key []byte, plaintext string) (string, error) {
	if plaintext == "" {
		return "", nil
	}
	if len(key) != KeySize {
		return "", fmt.Errorf("key must be %d bytes, got %d", KeySize, len(key))
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return "", fmt.Errorf("new cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("new gcm: %w", err)
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return "", fmt.Errorf("read nonce: %w", err)
	}

	ciphertext := gcm.Seal(nil, nonce, []byte(plaintext), nil)
	out := append(nonce, ciphertext...)
	return EncPrefix + base64.RawStdEncoding.EncodeToString(out), nil
}

// DecryptSecret расшифровывает строку, созданную EncryptSecret.
//
// Если строка не начинается с EncPrefix, возвращает ErrNotEncrypted
// (вызывающая сторона решает, считать ли это открытым текстом для
// миграции — но по умолчанию это означает «данные без шифрования»).
//
// Для пустой строки возвращает пустую строку без ошибки.
func DecryptSecret(key []byte, s string) (string, error) {
	if s == "" {
		return "", nil
	}
	if !IsEncrypted(s) {
		return "", ErrNotEncrypted
	}
	if len(key) != KeySize {
		return "", fmt.Errorf("key must be %d bytes, got %d", KeySize, len(key))
	}

	raw, err := base64.RawStdEncoding.DecodeString(strings.TrimPrefix(s, EncPrefix))
	if err != nil {
		return "", fmt.Errorf("decode base64: %w", err)
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return "", fmt.Errorf("new cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("new gcm: %w", err)
	}

	nonceSize := gcm.NonceSize()
	if len(raw) < nonceSize {
		return "", errors.New("ciphertext too short")
	}
	nonce, ciphertext := raw[:nonceSize], raw[nonceSize:]

	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", fmt.Errorf("decrypt: %w", err)
	}
	return string(plaintext), nil
}
