// SPDX-License-Identifier: AGPL-3.0-or-later

package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"database/sql"
	"encoding/base32"
	"encoding/hex"
	"errors"
	"fmt"
	"net/url"
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

// BuildOTPAuthURL собирает otpauth:// URL по существующему секрету.
//
// ВАЖНО: вызываем buildOTPAuthURLManual напрямую, БЕЗ totp.Generate.
// totp.Generate кодирует переданный Secret в base32 ещё раз, и QR
// получает другой секрет, чем в БД. Коды из приложения не совпадают.
// См. #5 нового аудита.
func BuildOTPAuthURL(username, secret string) (string, error) {
	return buildOTPAuthURLManual(username, secret), nil
}

// buildOTPAuthURLManual собирает otpauth URL вручную, если
// pquerna/otp не принимает существующий секрет.
func buildOTPAuthURLManual(username, secret string) string {
	label := url.PathEscape(Issuer + ":" + username)
	params := url.Values{}
	params.Set("secret", secret)
	params.Set("issuer", Issuer)
	params.Set("algorithm", "SHA1")
	params.Set("digits", "6")
	params.Set("period", "30")
	return "otpauth://totp/" + label + "?" + params.Encode()
}

// VerifyTOTP проверяет шестизначный код против секрета.
// Допускается окно ±1 шаг (30 секунд), чтобы компенсировать
// расхождение часов клиента и сервера.
// VerifyTOTP проверяет шестизначный код против секрета.
// Допускается окно ±1 шаг (30 секунд), чтобы компенсировать
// расхождение часов клиента и сервера.
//
// ВАЖНО: эта функция НЕ защищает от replay-атак. Один и тот же
// код можно использовать несколько раз в течение 90 секунд.
// Для защиты от replay используйте VerifyAndConsumeTOTP.
func VerifyTOTP(secret, code string) bool {
	_, ok := totpCounter(secret, code, time.Now().UTC())
	return ok
}

// totpCounter возвращает номер шага (counter), которому соответствует
// код, и true, если код валиден. Проверяет окно ±1 шаг от now.
//
// pquerna/otp не возвращает counter напрямую, поэтому перебираем
// три возможных шага вручную. Это те же три шага, что покрывает
// ValidateCustom с Skew: 1.
func totpCounter(secret, code string, now time.Time) (int64, bool) {
	code = strings.TrimSpace(code)
	if len(code) != 6 {
		return 0, false
	}

	step := now.Unix() / 30
	for _, delta := range []int64{0, -1, 1} {
		counter := step + delta
		expected, err := totp.GenerateCodeCustom(secret, time.Unix(counter*30, 0).UTC(), totp.ValidateOpts{
			Period:    30,
			Skew:      1,
			Digits:    otp.DigitsSix,
			Algorithm: otp.AlgorithmSHA1,
		})
		if err != nil {
			continue
		}
		if subtle.ConstantTimeCompare([]byte(expected), []byte(code)) == 1 {
			return counter, true
		}
	}
	return 0, false
}

// VerifyAndConsumeTOTP проверяет код и, если он валиден и его
// counter строго больше сохранённого для админа, обновляет
// totp_last_counter. Возвращает true, если код принят.
//
// Защищает от replay: один и тот же код (или код из предыдущего
// 30-секундного окна) не может быть использован дважды.
// См. P1-8 второго аудита.
func VerifyAndConsumeTOTP(conn *sql.DB, adminID int64, secret, code string) bool {
	if secret == "" || code == "" {
		return false
	}

	counter, ok := totpCounter(secret, code, time.Now().UTC())
	if !ok {
		return false
	}

	tx, err := conn.Begin()
	if err != nil {
		return false
	}
	defer tx.Rollback()

	var last int64
	if err := tx.QueryRow(
		`SELECT totp_last_counter FROM admins WHERE id = ?`,
		adminID,
	).Scan(&last); err != nil {
		return false
	}

	// Код должен быть из более позднего шага, чем уже использованный.
	if counter <= last {
		return false
	}

	if _, err := tx.Exec(
		`UPDATE admins SET totp_last_counter = ? WHERE id = ?`,
		counter, adminID,
	); err != nil {
		return false
	}

	return tx.Commit() == nil
}

// SaveTOTPSecret сохраняет секрет у администратора (включение 2FA).
// Секрет шифруется ключом из auth.SecretKey() перед записью.
func SaveTOTPSecret(conn *sql.DB, adminID int64, secret string) error {
	enc, err := EncryptSecret(SecretKey(), secret)
	if err != nil {
		return fmt.Errorf("encrypt totp secret: %w", err)
	}
	_, err = conn.Exec(
		`UPDATE admins SET totp_secret = ? WHERE id = ?`,
		enc, adminID,
	)
	if err != nil {
		return fmt.Errorf("save totp secret: %w", err)
	}
	return nil
}

// SavePendingTOTPSecret сохраняет «черновой» секрет, ещё не подтверждённый
// кодом из приложения. Не включается в totp_secret и не влияет на вход.
// Секрет шифруется перед записью.
func SavePendingTOTPSecret(conn *sql.DB, adminID int64, secret string) error {
	enc, err := EncryptSecret(SecretKey(), secret)
	if err != nil {
		return fmt.Errorf("encrypt pending totp secret: %w", err)
	}
	_, err = conn.Exec(
		`UPDATE admins SET totp_pending_secret = ? WHERE id = ?`,
		enc, adminID,
	)
	if err != nil {
		return fmt.Errorf("save pending totp secret: %w", err)
	}
	return nil
}

// GetPendingTOTPSecret возвращает черновой секрет (или пустую строку).
// Значение расшифровывается перед возвратом; наружу функция отдаёт
// plain text, хотя в БД хранится enc:v1:...
func GetPendingTOTPSecret(conn *sql.DB, adminID int64) (string, error) {
	var raw string
	err := conn.QueryRow(
		`SELECT totp_pending_secret FROM admins WHERE id = ?`,
		adminID,
	).Scan(&raw)
	if err != nil {
		return "", fmt.Errorf("get pending totp secret: %w", err)
	}
	if raw == "" {
		return "", nil
	}
	plain, err := DecryptSecret(SecretKey(), raw)
	if err != nil {
		return "", fmt.Errorf("decrypt pending totp secret: %w", err)
	}
	return plain, nil
}

// ErrTOTPAlreadyEnabled возвращается ActivatePendingTOTP, если у
// админа уже включена 2FA. Защита от подмены активного секрета
// через /2fa/setup + /2fa/enable без ведома владельца (сессия
// украдена) — см. P0-2 второго аудита.
var ErrTOTPAlreadyEnabled = errors.New("2FA is already enabled")

// ActivatePendingTOTP переносит черновой секрет в активный и очищает pending.
// Вызывается только после успешной проверки кода в /2fa/enable.
//
// Отказывается работать, если 2FA уже включена: ротация секрета
// должна идти через явный сценарий с подтверждением старого кода,
// а не через тихую подмену pending.
func ActivatePendingTOTP(conn *sql.DB, adminID int64) error {
	tx, err := conn.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	var pending, active string
	if err := tx.QueryRow(
		`SELECT totp_pending_secret, totp_secret FROM admins WHERE id = ?`,
		adminID,
	).Scan(&pending, &active); err != nil {
		return fmt.Errorf("get pending: %w", err)
	}
	if pending == "" {
		return errors.New("no pending totp secret")
	}
	if active != "" {
		return ErrTOTPAlreadyEnabled
	}

	if _, err := tx.Exec(
		`UPDATE admins SET totp_secret = ?, totp_pending_secret = '' WHERE id = ?`,
		pending, adminID,
	); err != nil {
		return fmt.Errorf("activate pending: %w", err)
	}
	return tx.Commit()
}

// ClearTOTPSecret отключает 2FA у администратора.
func ClearTOTPSecret(conn *sql.DB, adminID int64) error {
	tx, err := conn.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.Exec(
		`UPDATE admins SET totp_secret = '', totp_pending_secret = '' WHERE id = ?`,
		adminID,
	); err != nil {
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

// RecoveryCodeExists проверяет, что код восстановления существует
// и не был использован, БЕЗ его потребления. Нужен для обхода
// admin-lockout: на этом этапе пароль ещё не проверен, и тратить
// код нельзя. Потребление — через ConsumeRecoveryCode ниже, после
// успешной проверки пароля. См. P3-1 третьего аудита.
func RecoveryCodeExists(conn *sql.DB, adminID int64, code string) bool {
	code = strings.TrimSpace(strings.ToUpper(code))
	if code == "" {
		return false
	}
	hash := recoveryCodeHash(code)

	var n int
	err := conn.QueryRow(
		`SELECT COUNT(*) FROM recovery_codes
		 WHERE admin_id = ? AND code_hash = ? AND used_at = 0`,
		adminID, hash,
	).Scan(&n)
	if err != nil {
		return false
	}
	return n > 0
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
