// SPDX-License-Identifier: AGPL-3.0-or-later

// Package gopanel содержит вшитый фронтенд.
package gopanel

import (
	"embed"
	"io/fs"
)

//go:embed all:web/dist
var webFS embed.FS

// WebFS возвращает файловую систему с собранным фронтендом.
func WebFS() fs.FS {
	sub, err := fs.Sub(webFS, "web/dist")
	if err != nil {
		return nil
	}
	return sub
}
