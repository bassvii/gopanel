// SPDX-License-Identifier: AGPL-3.0-or-later

package system

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

// Пути к файлам GeoIP/GeoSite.
const (
	GeoDir      = "/usr/local/share/xray"
	GeoSiteFile = "geosite.dat"
	GeoIPFile   = "geoip.dat"
)

// Источники файлов (Loyalsoldier).
const (
	GeoSiteURL = "https://github.com/Loyalsoldier/v2ray-rules-dat/releases/latest/download/geosite.dat"
	GeoIPURL   = "https://github.com/Loyalsoldier/v2ray-rules-dat/releases/latest/download/geoip.dat"
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

	if err := downloadFile(ctx, GeoSiteURL, filepath.Join(GeoDir, GeoSiteFile)); err != nil {
		return updated, fmt.Errorf("geosite: %w", err)
	}
	updated++

	if err := downloadFile(ctx, GeoIPURL, filepath.Join(GeoDir, GeoIPFile)); err != nil {
		return updated, fmt.Errorf("geoip: %w", err)
	}
	updated++

	return updated, nil
}

// downloadFile скачивает URL в файл через временный файл.
func downloadFile(ctx context.Context, url, dst string) error {
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

	if err := os.Chmod(tmp, 0o644); err != nil {
		os.Remove(tmp)
		return err
	}

	return os.Rename(tmp, dst)
}
