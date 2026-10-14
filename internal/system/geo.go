// SPDX-License-Identifier: AGPL-3.0-or-later

package system

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Пути к файлам GeoIP/GeoSite.
const (
	GeoDir      = "/usr/local/share/xray"
	GeoSiteFile = "geosite.dat"
	GeoIPFile   = "geoip.dat"
)

// Версия geosite/geoip (Loyalsoldier/v2ray-rules-dat).
// Синхронизируется с deploy/versions.env (GEO_TAG).
// Обновлять вручную: взять свежий тег с GitHub, посчитать sha256,
// обновить обе константы ниже и в deploy/versions.env.
const (
	GeoTag = "202610132205"

	// SHA-256 файлов этого релиза.
	GeoSiteSHA256 = "c5a73fbd415bb0506ecf8fa574caadc6f54a71d9308c265e97da49e4cebf8466"  // ← сюда хеш geosite.dat
	GeoIPSHA256   = "116cc0f03d48991962f7f9cdbbcf53d45d89777f9c3cd1a663210853e5df6093"  // ← сюда хеш geoip.dat
)

// Источники файлов (Loyalsoldier), зафиксированы на GeoTag.
var (
	GeoSiteURL = "https://github.com/Loyalsoldier/v2ray-rules-dat/releases/download/" + GeoTag + "/geosite.dat"
	GeoIPURL   = "https://github.com/Loyalsoldier/v2ray-rules-dat/releases/download/" + GeoTag + "/geoip.dat"
)

// GeoFileInfo — информация о файле.
type GeoFileInfo struct {
	Name     string `json:"name"`
	Path     string `json:"path"`
	Exists   bool   `json:"exists"`
	SizeBytes int64 `json:"size_bytes"`
	ModifiedAt int64 `json:"modified_at"`
}

// GeoStatus — состояние GeoIP/GeoSite.
type GeoStatus struct {
	Dir      string        `json:"dir"`
	GeoSite  GeoFileInfo   `json:"geosite"`
	GeoIP    GeoFileInfo   `json:"geoip"`
}

// GetGeoStatus возвращает информацию о файлах.
func GetGeoStatus() GeoStatus {
	return GeoStatus{
		Dir:     GeoDir,
		GeoSite: fileInfo(filepath.Join(GeoDir, GeoSiteFile), GeoSiteFile),
		GeoIP:   fileInfo(filepath.Join(GeoDir, GeoIPFile), GeoIPFile),
	}
}

func fileInfo(path, name string) GeoFileInfo {
	info := GeoFileInfo{Name: name, Path: path}
	if st, err := os.Stat(path); err == nil {
		info.Exists = true
		info.SizeBytes = st.Size()
		info.ModifiedAt = st.ModTime().Unix()
	}
	return info
}

// UpdateGeo скачивает свежие файлы GeoIP/GeoSite.
// Возвращает число обновлённых файлов.
func UpdateGeo(ctx context.Context) (int, error) {
	if err := os.MkdirAll(GeoDir, 0o755); err != nil {
		return 0, fmt.Errorf("mkdir %s: %w", GeoDir, err)
	}

	updated := 0

	if err := downloadFile(ctx, GeoSiteURL, GeoSiteSHA256, filepath.Join(GeoDir, GeoSiteFile)); err != nil {
		return updated, fmt.Errorf("geosite: %w", err)
	}
	updated++

	if err := downloadFile(ctx, GeoIPURL, GeoIPSHA256, filepath.Join(GeoDir, GeoIPFile)); err != nil {
		return updated, fmt.Errorf("geoip: %w", err)
	}
	updated++

	return updated, nil
}

// downloadFile скачивает URL во временный файл, проверяет SHA-256,
// и только потом перемещает в целевой путь. При несовпадении —
// удаляет временный файл и возвращает ошибку.
func downloadFile(ctx context.Context, url, expectedSHA, dst string) error {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("http %d", resp.StatusCode)
	}

	tmp := dst + ".tmp"
	f, err := os.Create(tmp)
	if err != nil {
		return err
	}
	defer f.Close()

	if _, err := io.Copy(f, resp.Body); err != nil {
		os.Remove(tmp)
		return err
	}
	f.Close()

	// Проверка SHA-256 перед заменой целевого файла.
	if err := verifySHA256(tmp, expectedSHA); err != nil {
		os.Remove(tmp)
		return fmt.Errorf("sha256 check failed: %w", err)
	}

	if err := os.Chmod(tmp, 0o644); err != nil {
		os.Remove(tmp)
		return err
	}

	return os.Rename(tmp, dst)
}

// verifySHA256 считает sha256 файла и сравнивает с ожидаемым (hex).
// Сравнение константное по времени через subtle.ConstantTimeCompare.
func verifySHA256(path, expected string) error {
	expected = strings.ToLower(strings.TrimSpace(expected))
	if expected == "" {
		return errors.New("expected sha256 is empty")
	}

	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return err
	}
	actual := hex.EncodeToString(h.Sum(nil))

	if subtle.ConstantTimeCompare([]byte(actual), []byte(expected)) != 1 {
		return fmt.Errorf("expected %s, got %s", expected, actual)
	}
	return nil
}
