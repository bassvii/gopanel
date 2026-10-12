// SPDX-License-Identifier: AGPL-3.0-or-later

// Package system читает системные метрики Linux из /proc.
package system

import (
	"fmt"
	"os"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
)

// Stats — текущие метрики системы.
type Stats struct {
	CPUPercent    float64 `json:"cpu_percent"`
	CPUCores      int     `json:"cpu_cores"`
	MemUsedBytes  int64   `json:"mem_used_bytes"`
	MemTotalBytes int64   `json:"mem_total_bytes"`
	MemPercent    float64 `json:"mem_percent"`
	DiskUsedBytes  int64  `json:"disk_used_bytes"`
	DiskTotalBytes int64  `json:"disk_total_bytes"`
	DiskPercent   float64 `json:"disk_percent"`
	LoadAvg1      float64 `json:"load_avg_1"`
	LoadAvg5      float64 `json:"load_avg_5"`
	LoadAvg15     float64 `json:"load_avg_15"`
	UptimeSeconds int64   `json:"uptime_seconds"`
	Timestamp     int64   `json:"ts"`
}

// Point — одна точка истории для графиков.
type Point struct {
	Timestamp  int64   `json:"ts"`
	CPUPercent float64 `json:"cpu_percent"`
	MemPercent float64 `json:"mem_percent"`
}

// History — кольцевой буфер точек.
type History struct {
	mu     sync.Mutex
	points []Point
	max    int
}

func NewHistory(max int) *History {
	if max <= 0 {
		max = 720 // 1 час при интервале 5 сек
	}
	return &History{max: max}
}

func (h *History) Add(p Point) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.points = append(h.points, p)
	if len(h.points) > h.max {
		h.points = h.points[len(h.points)-h.max:]
	}
}

func (h *History) Snapshot() []Point {
	h.mu.Lock()
	defer h.mu.Unlock()
	out := make([]Point, len(h.points))
	copy(out, h.points)
	return out
}

// cpuSample — предыдущий замер CPU.
var lastCPU struct {
	mu     sync.Mutex
	total  uint64
	idle   uint64
	valid  bool
}

// ReadStats собирает текущие метрики.
// diskPath — путь для измерения диска (обычно "/").
func ReadStats(diskPath string) (Stats, error) {
	var s Stats
	s.Timestamp = time.Now().Unix()
	s.CPUCores = runtime.NumCPU()

	// CPU
	cpuPct, err := readCPUPercent()
	if err == nil {
		s.CPUPercent = cpuPct
	}

	// Память
	memTotal, memUsed, err := readMem()
	if err == nil {
		s.MemTotalBytes = memTotal
		s.MemUsedBytes = memUsed
		if memTotal > 0 {
			s.MemPercent = float64(memUsed) / float64(memTotal) * 100
		}
	}

	// Диск
	diskTotal, diskUsed, err := readDisk(diskPath)
	if err == nil {
		s.DiskTotalBytes = diskTotal
		s.DiskUsedBytes = diskUsed
		if diskTotal > 0 {
			s.DiskPercent = float64(diskUsed) / float64(diskTotal) * 100
		}
	}

	// Load average
	l1, l5, l15, err := readLoadAvg()
	if err == nil {
		s.LoadAvg1 = l1
		s.LoadAvg5 = l5
		s.LoadAvg15 = l15
	}

	// Uptime
	uptime, err := readUptime()
	if err == nil {
		s.UptimeSeconds = uptime
	}

	return s, nil
}

// --- CPU ---

func readCPUPercent() (float64, error) {
	data, err := os.ReadFile("/proc/stat")
	if err != nil {
		return 0, err
	}

	var total, idle uint64
	var found bool
	for _, line := range strings.Split(string(data), "\n") {
		if !strings.HasPrefix(line, "cpu ") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 5 {
			return 0, fmt.Errorf("unexpected /proc/stat format")
		}
		for i, f := range fields[1:] {
			v, err := strconv.ParseUint(f, 10, 64)
			if err != nil {
				continue
			}
			total += v
			if i == 3 || i == 4 {
				idle += v
			}
		}
		found = true
		break
	}
	if !found {
		return 0, fmt.Errorf("no cpu line in /proc/stat")
	}

	lastCPU.mu.Lock()
	defer lastCPU.mu.Unlock()

	var pct float64
	if lastCPU.valid {
		dTotal := total - lastCPU.total
		dIdle := idle - lastCPU.idle
		if dTotal > 0 {
			pct = float64(dTotal-dIdle) / float64(dTotal) * 100
		}
	}
	lastCPU.total = total
	lastCPU.idle = idle
	lastCPU.valid = true
	return pct, nil
}

// --- Memory ---

func readMem() (total int64, used int64, err error) {
	data, err := os.ReadFile("/proc/meminfo")
	if err != nil {
		return 0, 0, err
	}
	var memTotal, memAvailable int64
	for _, line := range strings.Split(string(data), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		v, _ := strconv.ParseInt(fields[1], 10, 64)
		switch fields[0] {
		case "MemTotal:":
			memTotal = v * 1024
		case "MemAvailable:":
			memAvailable = v * 1024
		}
	}
	if memTotal == 0 {
		return 0, 0, fmt.Errorf("no MemTotal in /proc/meminfo")
	}
	used = memTotal - memAvailable
	return memTotal, used, nil
}

// --- Disk ---

func readDisk(path string) (total int64, used int64, err error) {
	var st syscall.Statfs_t
	if err := syscall.Statfs(path, &st); err != nil {
		return 0, 0, err
	}
	total = int64(st.Blocks) * int64(st.Bsize)
	free := int64(st.Bavail) * int64(st.Bsize)
	used = total - free
	return total, used, nil
}

// --- Load average ---

func readLoadAvg() (l1, l5, l15 float64, err error) {
	data, err := os.ReadFile("/proc/loadavg")
	if err != nil {
		return 0, 0, 0, err
	}
	fields := strings.Fields(string(data))
	if len(fields) < 3 {
		return 0, 0, 0, fmt.Errorf("unexpected /proc/loadavg format")
	}
	l1, _ = strconv.ParseFloat(fields[0], 64)
	l5, _ = strconv.ParseFloat(fields[1], 64)
	l15, _ = strconv.ParseFloat(fields[2], 64)
	return l1, l5, l15, nil
}

// --- Uptime ---

func readUptime() (int64, error) {
	data, err := os.ReadFile("/proc/uptime")
	if err != nil {
		return 0, err
	}
	fields := strings.Fields(string(data))
	if len(fields) < 1 {
		return 0, fmt.Errorf("unexpected /proc/uptime format")
	}
	seconds, _ := strconv.ParseFloat(fields[0], 64)
	return int64(seconds), nil
}
