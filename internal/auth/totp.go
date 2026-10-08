// SPDX-License-Identifier: AGPL-3.0-or-later

package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base32"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/pquerna/otp"
	"github.com/pquerna/otp/totp"
)

// Issuer — имя, отображаемое в приложении-аутентификаторе.
const Issuer = "gopanel"

// RecoveryCodeCount — сколько кодов восстановления генерируется.
const RecoveryCodeCount = 10

// GenerateTOTPSecret создаёт новый TOTP-секрет для администратора.
// Возвращает секрет в base32 и otpauth-URL для QR.
func GenerateTOTPSecret(username string) (string, string, error) {
	key, err := totp.Generate(totp.GenerateOpts{
		Issuer:      Issuer,
		AccountName: username,
		Period:      30,
		SecretSize:  20,
		Digits:      otp.DigitsSix,
		Algorithm:   otp.AlgorithmSHA1,
	})
	if err != nil {
		return "", "", fmt.Errorf("generate totp: %w", err)
	}
	return key.Secret(), key.URL(), nil
}

// VerifyTOTP проверяет шестизначный код против секрета.
// Допускается окно ±1 шаг (30 секунд), чтобы компенсировать
// расхождение часов клиента и сервера.
func VerifyTOTP(secret, code string) bool {
	code = strings.TrimSpace(code)
	if len(code) != 6 {
		return false
	}
	valid, err := totp.ValidateCustom(code, secret, time.Now().UTC(), totp.ValidateOpts{
		Period:    30,
		Skew:      1,
		Digits:    otp.DigitsSix,
		Algorithm: otp.AlgorithmSHA1,
	})
	if err != nil {
		return false
	}
	return valid
}

// SaveTOTPSecret сохраняет секрет у администратора (включение 2FA).
func SaveTOTPSecret(conn *sql.DB, adminID int64, secret string) error {
	_, err := conn.Exec(
		`UPDATE admins SET totp_secret = ? WHERE id = ?`,
		secret, adminID,
	)
	if err != nil {
		return fmt.Errorf("save totp secret: %w", err)
	}
	return nil
}

// ClearTOTPSecret отключает 2FA у администратора.
func ClearTOTPSecret(conn *sql.DB, adminID int64) error {
	tx, err := conn.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.Exec(`UPDATE admins SET totp_secret = '' WHERE id = ?`, adminID); err != nil {
		return err
	}
	if _, err := tx.Exec(`DELETE FROM recovery_codes WHERE admin_id = ?`, adminID); err != nil {
		return err
	}
	return tx.Commit()
}

// GenerateRecoveryCodes создаёт набор кодов восстановления,
// сохраняет их хеши и возвращает коды в открытом виде (один раз).
func GenerateRecoveryCodes(conn *sql.DB, adminID int64) ([]string, error) {
	tx, err := conn.Begin()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	if _, err := tx.Exec(`DELETE FROM recovery_codes WHERE admin_id = ?`, adminID); err != nil {
		return nil, err
	}

	codes := make([]string, 0, RecoveryCodeCount)
	for i := 0; i < RecoveryCodeCount; i++ {
		code, err := randomRecoveryCode()
		if err != nil {
			return nil, err
		}
		codes = append(codes, code)

		if _, err := tx.Exec(
			`INSERT INTO recovery_codes (admin_id, code_hash) VALUES (?, ?)`,
			adminID, recoveryCodeHash(code),
		); err != nil {
			return nil, err
		}
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return codes, nil
}

// ConsumeRecoveryCode проверяет код восстановления и помечает его
// использованным. Возвращает nil, если код подошёл и не был использован.
func ConsumeRecoveryCode(conn *sql.DB, adminID int64, code string) error {
	code = strings.TrimSpace(strings.ToUpper(code))
	if code == "" {
		return errors.New("empty code")
	}
	hash := recoveryCodeHash(code)

	res, err := conn.Exec(
		`UPDATE recovery_codes SET used_at = ?
		 WHERE admin_id = ? AND code_hash = ? AND used_at = 0`,
		time.Now().Unix(), adminID, hash,
	)
	if err != nil {
		return fmt.Errorf("consume recovery code: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return errors.New("invalid or used recovery code")
	}
	return nil
}

func randomRecoveryCode() (string, error) {
	raw := make([]byte, 10)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	enc := base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(raw)
	// Формат: XXXX-XXXX-XXXX-XXXX (20 символов base32 → 16 с дефисами).
	return enc[0:4] + "-" + enc[4:8] + "-" + enc[8:12] + "-" + enc[12:16], nil
}

func recoveryCodeHash(code string) string {
	sum := sha256.Sum256([]byte(code))
	return hex.EncodeToString(sum[:])
}
