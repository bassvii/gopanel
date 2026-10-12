// SPDX-License-Identifier: AGPL-3.0-or-later

package api

import (
	"net/http"
	"os"
	"time"
)

type systemResponse struct {
	Current systemCurrent   `json:"current"`
	History []systemPoint   `json:"history"`
	Xray    xrayStatus      `json:"xray"`
	Panel   panelStatus     `json:"panel"`
}

type systemCurrent struct {
	CPUPercent     float64 `json:"cpu_percent"`
	CPUCores       int     `json:"cpu_cores"`
	MemUsedBytes   int64   `json:"mem_used_bytes"`
	MemTotalBytes  int64   `json:"mem_total_bytes"`
	MemPercent     float64 `json:"mem_percent"`
	DiskUsedBytes  int64   `json:"disk_used_bytes"`
	DiskTotalBytes int64   `json:"disk_total_bytes"`
	DiskPercent    float64 `json:"disk_percent"`
	LoadAvg1       float64 `json:"load_avg_1"`
	LoadAvg5       float64 `json:"load_avg_5"`
	LoadAvg15      float64 `json:"load_avg_15"`
	UptimeSeconds  int64   `json:"uptime_seconds"`
	Timestamp      int64   `json:"ts"`
}

type systemPoint struct {
	Timestamp  int64   `json:"ts"`
	CPUPercent float64 `json:"cpu_percent"`
	MemPercent float64 `json:"mem_percent"`
}

type xrayStatus struct {
	Running       bool   `json:"running"`
	Version       string `json:"version"`
	UptimeSeconds int64  `json:"uptime_seconds"`
}

type panelStatus struct {
	Version       string `json:"version"`
	UptimeSeconds int64  `json:"uptime_seconds"`
	DBSizeBytes   int64  `json:"db_size_bytes"`
}

// handleSystemStats — GET /system/stats
func (s *Server) handleSystemStats(w http.ResponseWriter, r *http.Request) {
	s.log.Info("sysstats: begin")
	defer s.log.Info("sysstats: end")

	if _, ok := s.requireSession(w, r); !ok {
		s.log.Info("sysstats: session failed")
		return
	}
	s.log.Info("sysstats: session ok")

	resp := systemResponse{}

	if s.system != nil {
		s.log.Info("sysstats: reading current")
		cur, err := s.system.Current()
		if err == nil {
			resp.Current = systemCurrent{
				CPUPercent:     cur.CPUPercent,
				CPUCores:       cur.CPUCores,
				MemUsedBytes:   cur.MemUsedBytes,
				MemTotalBytes:  cur.MemTotalBytes,
				MemPercent:     cur.MemPercent,
				DiskUsedBytes:  cur.DiskUsedBytes,
				DiskTotalBytes: cur.DiskTotalBytes,
				DiskPercent:    cur.DiskPercent,
				LoadAvg1:       cur.LoadAvg1,
				LoadAvg5:       cur.LoadAvg5,
				LoadAvg15:      cur.LoadAvg15,
				UptimeSeconds:  cur.UptimeSeconds,
				Timestamp:      cur.Timestamp,
			}
		} else {
			s.log.Warn("sysstats: current failed", "err", err)
		}
		s.log.Info("sysstats: reading history")
		pts := s.system.History()
		resp.History = make([]systemPoint, 0, len(pts))
		for _, p := range pts {
			resp.History = append(resp.History, systemPoint{
				Timestamp:  p.Timestamp,
				CPUPercent: p.CPUPercent,
				MemPercent: p.MemPercent,
			})
		}
		s.log.Info("sysstats: history done", "points", len(pts))
	}

	s.log.Info("sysstats: xray")

	if s.core != nil {
		s.log.Info("sysstats: xray IsRunning")
		resp.Xray.Running = s.core.IsRunning()
		s.log.Info("sysstats: xray IsRunning done", "running", resp.Xray.Running)

		s.log.Info("sysstats: xray Version")
		if v, err := s.core.Version(r.Context()); err == nil {
			resp.Xray.Version = v
		} else {
			s.log.Warn("sysstats: xray Version failed", "err", err)
		}
		s.log.Info("sysstats: xray Version done")

		if resp.Xray.Running {
			s.log.Info("sysstats: xray UptimeSeconds")
			resp.Xray.UptimeSeconds = s.core.UptimeSeconds()
			s.log.Info("sysstats: xray UptimeSeconds done")
		}
	}

	s.log.Info("sysstats: panel")
	resp.Panel.Version = "0.1"
	if !s.startTime.IsZero() {
		resp.Panel.UptimeSeconds = int64(time.Since(s.startTime).Seconds())
	}
	if s.dbPath != "" {
		if info, err := os.Stat(s.dbPath); err == nil {
			resp.Panel.DBSizeBytes = info.Size()
		}
	}

	s.log.Info("sysstats: write")
	writeJSON(w, http.StatusOK, resp)
}
