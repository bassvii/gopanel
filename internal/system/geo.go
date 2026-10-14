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

// Имена файлов GeoIP/GeoSite.
const (
	GeoSiteFile = "geosite.dat"
	GeoIPFile   = "geoip.dat"
)

// DefaultGeoDir — путь по умолчанию, если GOPANEL_GEO_DIR не задан.
//
// ВАЖНО: этот путь должен быть доступен на запись процессу панели.
// Под systemd unit использует ProtectSystem=strict +
// ReadWritePaths=/var/lib/gopanel. В Docker — volume на
// /var/lib/gopanel. Поэтому дефолт — рядом с данными, а не
// в /usr/local/share/xray (там писать нельзя — см. P2-1 аудита).
//
// Историческая заметка: раньше здесь был /usr/local/share/xray.
// При обновлении с версии < 0.1 нужно либо скопировать файлы
// вручную, либо позволить панели скачать их заново через
// POST /system/geo/update.
var DefaultGeoDir = "/var/lib/gopanel/xray"

// GeoDir возвращает каталог для geo-файлов: из env GOPANEL_GEO_DIR,
// иначе DefaultGeoDir.
func GeoDir() string {
	if v := os.Getenv("GOPANEL_GEO_DIR"); v != "" {
		return v
	}
	return DefaultGeoDir
}

// Версия geosite/geoip (Loyalsoldier/v2ray-rules-dat).
// Синхронизируется с deploy/versions.env (GEO_TAG).
// Обновлять вручную: взять свежий тег с GitHub, посчитать sha256,
// обновить обе константы ниже и в deploy/versions.env.
const (
	GeoTag = "202610132205"

	// SHA-256 файлов этого релиза.
	GeoSiteSHA256 = "c5a73fbd415bb0506ecf8fa574caadc6f54a71d9308c265e97da49e4cebf8466" // ← сюда хеш geosite.dat
	GeoIPSHA256   = "116cc0f03d48991962f7f9cdbbcf53d45d89777f9c3cd1a663210853e5df6093" // ← сюда хеш geoip.dat
)

// Источники файлов (Loyalsoldier), зафиксированы на GeoTag.
var (
	GeoSiteURL = "https://github.com/Loyalsoldier/v2ray-rules-dat/releases/download/" + GeoTag + "/geosite.dat"
	GeoIPURL   = "https://github.com/Loyalsoldier/v2ray-rules-dat/releases/download/" + GeoTag + "/geoip.dat"
)

// GeoFileInfo — информация о файле.
type GeoFileInfo struct {
	Name       string `json:"name"`
	Path       string `json:"path"`
	Exists     bool   `json:"exists"`
	SizeBytes  int64  `json:"size_bytes"`
	ModifiedAt int64  `json:"modified_at"`
}

// GeoStatus — состояние GeoIP/GeoSite.
type GeoStatus struct {
	Dir     string      `json:"dir"`
	GeoSite GeoFileInfo `json:"geosite"`
	GeoIP   GeoFileInfo `json:"geoip"`
}

// GetGeoStatus возвращает информацию о файлах.
func GetGeoStatus() GeoStatus {
	dir := GeoDir()
	return GeoStatus{
		Dir:     dir,
		GeoSite: fileInfo(filepath.Join(dir, GeoSiteFile), GeoSiteFile),
		GeoIP:   fileInfo(filepath.Join(dir, GeoIPFile), GeoIPFile),
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
	dir := GeoDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return 0, fmt.Errorf("mkdir %s: %w", dir, err)
	}

	updated := 0

	if err := downloadFile(ctx, GeoSiteURL, GeoSiteSHA256, filepath.Join(dir, GeoSiteFile)); err != nil {
		return updated, fmt.Errorf("geosite: %w", err)
	}
	updated++

	if err := downloadFile(ctx, GeoIPURL, GeoIPSHA256, filepath.Join(dir, GeoIPFile)); err != nil {
		return updated, fmt.Errorf("geoip: %w", err)
	}
	updated++

	return updated, nil
}

// maxGeoFileSize — потолок на размер скачиваемого geo-файла.
// Реальные geosite.dat/geoip.dat — десятки мегабайт; 100 МиБ
// с запасом. Защита от подмены источника или редиректа на
// бесконечный поток. См. P3-6 третьего аудита.
const maxGeoFileSize = 100 * 1024 * 1024 // 100 MiB

// httpClient — свой клиент с таймаутом и без бесконечных редиректов.
// Раньше использовался http.DefaultClient без ограничений.
var httpClient = &http.Client{
	Timeout: 5 * time.Minute,
	CheckRedirect: func(req *http.Request, via []*http.Request) error {
		if len(via) >= 5 {
			return fmt.Errorf("too many redirects (%d)", len(via))
		}
		return nil
	},
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

	resp, err := httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("http %d", resp.StatusCode)
	}

	// Лимит на размер: io.LimitReader +1 байт, чтобы отличить
	// «ровно в лимит» от «больше лимита».
	limited := io.LimitReader(resp.Body, maxGeoFileSize+1)

	tmp := dst + ".tmp"
	f, err := os.Create(tmp)
	if err != nil {
		return err
	}

	written, err := io.Copy(f, limited)
	if err != nil {
		f.Close()
		os.Remove(tmp)
		return err
	}
	if written > maxGeoFileSize {
		f.Close()
		os.Remove(tmp)
		return fmt.Errorf("geo file exceeds %d bytes", maxGeoFileSize)
	}
	if err := f.Close(); err != nil {
		os.Remove(tmp)
		return err
	}

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
