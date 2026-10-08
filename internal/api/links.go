// SPDX-License-Identifier: AGPL-3.0-or-later

package api

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/bassvii/gopanel/internal/db"
	"github.com/bassvii/gopanel/internal/links"
)

type linkEntry struct {
	InboundID int64  `json:"inbound_id"`
	Tag       string `json:"tag"`
	Protocol  string `json:"protocol"`
	Remark    string `json:"remark"`
	URL       string `json:"url"`
	QRBase64  string `json:"qr_base64,omitempty"`
	Error     string `json:"error,omitempty"`
}

type linksResponse struct {
	UserID int64       `json:"user_id"`
	Links  []linkEntry `json:"links"`
}

// handleUserLinks — GET /users/{id}/links?address=...&qr=1
func (s *Server) handleUserLinks(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireSession(w, r); !ok {
		return
	}

	// Извлечь userID из пути: /users/{id}/links
	path := strings.TrimPrefix(r.URL.Path, s.secret+"/users/")
	parts := strings.Split(path, "/")
	if len(parts) < 2 || parts[1] != "links" {
		http.Error(w, "invalid path", http.StatusBadRequest)
		return
	}
	userID, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil {
		http.Error(w, "invalid user id", http.StatusBadRequest)
		return
	}

	// Адрес сервера: query-параметр или дефолт.
	address := r.URL.Query().Get("address")
	if address == "" {
		address = "127.0.0.1"
	}
	withQR := r.URL.Query().Get("qr") == "1"

	user, err := db.GetUser(s.db, userID)
	if errors.Is(err, db.ErrNotFound) {
		http.NotFound(w, r)
		return
	}
	if err != nil {
		s.log.Error("get user failed", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	// Все связи пользователя.
	userInbounds, err := db.ListInboundsForUser(s.db, userID)
	if err != nil {
		s.log.Error("list user inbounds failed", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	resp := linksResponse{UserID: userID}
	for _, ui := range userInbounds {
		in, err := db.GetInbound(s.db, ui.InboundID)
		if err != nil {
			resp.Links = append(resp.Links, linkEntry{
				InboundID: ui.InboundID,
				Error:     "inbound not found",
			})
			continue
		}

		remark := user.Name + " · " + in.Tag
		url, err := links.Generate(links.Input{
			Protocol:       in.Protocol,
			Tag:            in.Tag,
			Address:        address,
			Port:           in.Port,
			CredentialJSON: ui.CredentialJSON,
			SettingsJSON:   in.SettingsJSON,
			StreamJSON:     in.StreamJSON,
			Remark:         remark,
		})
		if err != nil {
			resp.Links = append(resp.Links, linkEntry{
				InboundID: ui.InboundID,
				Tag:       in.Tag,
				Protocol:  in.Protocol,
				Remark:    remark,
				Error:     err.Error(),
			})
			continue
		}

		entry := linkEntry{
			InboundID: ui.InboundID,
			Tag:       in.Tag,
			Protocol:  in.Protocol,
			Remark:    remark,
			URL:       url,
		}

		if withQR {
			qr, err := links.QRPNGBase64(url, 256)
			if err != nil {
				entry.Error = "qr generation failed"
			} else {
				entry.QRBase64 = qr
			}
		}

		resp.Links = append(resp.Links, entry)
	}

	writeJSON(w, http.StatusOK, resp)
}
