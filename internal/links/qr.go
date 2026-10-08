// SPDX-License-Identifier: AGPL-3.0-or-later

package links

import (
	"encoding/base64"
	"fmt"

	qrcode "github.com/skip2/go-qrcode"
)

// QRPNGBase64 возвращает QR-код ссылки как PNG в base64 (без префикса data:).
// size — сторона изображения в пикселях (обычно 256).
func QRPNGBase64(content string, size int) (string, error) {
	if size <= 0 {
		size = 256
	}
	png, err := qrcode.Encode(content, qrcode.Medium, size)
	if err != nil {
		return "", fmt.Errorf("qrcode encode: %w", err)
	}
	return base64.StdEncoding.EncodeToString(png), nil
}
