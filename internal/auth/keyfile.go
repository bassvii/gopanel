// SPDX-License-Identifier: AGPL-3.0-or-later

package auth

import (
	"crypto/rand"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

// KeySize — длина ключа шифрования (AES-256).
const KeySize = 32

// KeyPath возвращает путь к файлу ключа рядом с БД.
// Например, для /var/lib/gopanel/gopanel.db вернёт
// /var/lib/gopanel/secret.key.
func KeyPath(dbPath string) string {
	dir := filepath.Dir(dbPath)
	if dir == "" {
		dir = "."
	}
	return filepath.Join(dir, "secret.key")
}

// LoadOrCreateKey читает 32-байтный ключ из файла path.
// Если файла нет — создаёт его с правами 0600 и случайным содержимым.
// Если файл есть, но повреждён (не 32 байта) — возвращает ошибку.
func LoadOrCreateKey(path string) ([]byte, error) {
	data, err := os.ReadFile(path)
	if err == nil {
		if len(data) != KeySize {
			return nil, fmt.Errorf("secret key %s: expected %d bytes, got %d",
				path, KeySize, len(data))
		}
		return data, nil
	}
	if !errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("read secret key %s: %w", path, err)
	}

	// Создаём новый ключ атомарно: пишем во временный файл, потом rename.
	key := make([]byte, KeySize)
	if _, err := rand.Read(key); err != nil {
		return nil, fmt.Errorf("generate key: %w", err)
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, fmt.Errorf("mkdir for key: %w", err)
	}

	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, key, 0o600); err != nil {
		return nil, fmt.Errorf("write key tmp: %w", err)
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return nil, fmt.Errorf("rename key: %w", err)
	}
	return key, nil
}

var (
	globalKey   []byte
	globalKeyMu sync.RWMutex
)

// SetSecretKey устанавливает глобальный ключ шифрования.
// Вызывается один раз из db.Open при старте панели.
func SetSecretKey(key []byte) {
	globalKeyMu.Lock()
	defer globalKeyMu.Unlock()
	globalKey = key
}

// SecretKey возвращает установленный ключ (или nil).
func SecretKey() []byte {
	globalKeyMu.RLock()
	defer globalKeyMu.RUnlock()
	return globalKey
}
