package main

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"time"
)

// watchedServices are the systemd units the health check watches on prod.
var watchedServices = []string{"jarvis", "jarvis-support", "caddy", "postgresql"}

// healthStatePath persists the last-known problem-set hash so healthcheck
// only alerts on state change. The directory already exists on prod;
// writing fails soft everywhere else (e.g. local macOS dev).
const healthStatePath = "/var/lib/jarvis/health-state"

// ServiceState is one watched systemd unit's reported state ("active",
// "inactive", "failed", or "n/a" when systemctl itself is unavailable).
type ServiceState struct {
	Name  string `json:"name"`
	State string `json:"state"`
}

// SystemStatus is the live host/service snapshot behind the HUD's System
// panel and the healthcheck/brief. Numeric fields use -1 to mean
// "unavailable" (e.g. /proc missing on macOS dev) rather than a false 0.
type SystemStatus struct {
	Hostname string         `json:"hostname"`
	Uptime   string         `json:"uptime"` // formatted, "n/a" if unavailable
	Load1    float64        `json:"load1"`
	Load5    float64        `json:"load5"`
	Load15   float64        `json:"load15"`
	MemPct   float64        `json:"mem_pct"`
	DiskPct  float64        `json:"disk_pct"`
	NumCPU   int            `json:"num_cpu"`
	Services []ServiceState `json:"services"`
	DBOnline bool           `json:"db_online"`
	SiteOK   bool           `json:"site_ok"`
}

// collectSystemStatus gathers a live host snapshot. Every probe fails soft:
// a missing /proc, a down service, or an unreachable site never panics or
// blocks — the field just reads as "unavailable" (-1 / "n/a" / false).
func collectSystemStatus(ctx context.Context, db *sql.DB) SystemStatus {
	s := SystemStatus{Load1: -1, Load5: -1, Load15: -1, MemPct: -1, DiskPct: -1, NumCPU: runtime.NumCPU()}

	if h, err := os.Hostname(); err == nil {
		s.Hostname = h
	} else {
		s.Hostname = "n/a"
	}

	s.Uptime = "n/a"
	if data, err := os.ReadFile("/proc/uptime"); err == nil {
		if d, ok := parseUptimeSeconds(string(data)); ok {
			s.Uptime = formatUptime(d)
		}
	}

	if data, err := os.ReadFile("/proc/loadavg"); err == nil {
		if l1, l5, l15, ok := parseLoadAvg(string(data)); ok {
			s.Load1, s.Load5, s.Load15 = l1, l5, l15
		}
	}

	if data, err := os.ReadFile("/proc/meminfo"); err == nil {
		if pct, ok := parseMemInfo(string(data)); ok {
			s.MemPct = pct
		}
	}

	if pct, ok := diskUsedPct("/"); ok {
		s.DiskPct = pct
	}

	for _, name := range watchedServices {
		s.Services = append(s.Services, ServiceState{Name: name, State: serviceActive(ctx, name)})
	}

	if db != nil {
		dctx, cancel := context.WithTimeout(ctx, 3*time.Second)
		s.DBOnline = db.PingContext(dctx) == nil
		cancel()
	}

	s.SiteOK = checkSite(ctx)

	return s
}

// parseMemInfo reads MemTotal/MemAvailable out of /proc/meminfo content and
// returns the used-memory percentage.
func parseMemInfo(data string) (pct float64, ok bool) {
	var total, avail float64
	found := 0
	for _, line := range strings.Split(data, "\n") {
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		switch fields[0] {
		case "MemTotal:":
			if v, err := strconv.ParseFloat(fields[1], 64); err == nil {
				total = v
				found++
			}
		case "MemAvailable:":
			if v, err := strconv.ParseFloat(fields[1], 64); err == nil {
				avail = v
				found++
			}
		}
	}
	if found < 2 || total == 0 {
		return 0, false
	}
	return (total - avail) / total * 100, true
}

// parseLoadAvg reads the three load averages out of /proc/loadavg content.
func parseLoadAvg(data string) (load1, load5, load15 float64, ok bool) {
	fields := strings.Fields(data)
	if len(fields) < 3 {
		return 0, 0, 0, false
	}
	var err error
	if load1, err = strconv.ParseFloat(fields[0], 64); err != nil {
		return 0, 0, 0, false
	}
	if load5, err = strconv.ParseFloat(fields[1], 64); err != nil {
		return 0, 0, 0, false
	}
	if load15, err = strconv.ParseFloat(fields[2], 64); err != nil {
		return 0, 0, 0, false
	}
	return load1, load5, load15, true
}

// parseUptimeSeconds reads the first field of /proc/uptime content.
func parseUptimeSeconds(data string) (time.Duration, bool) {
	fields := strings.Fields(data)
	if len(fields) < 1 {
		return 0, false
	}
	secs, err := strconv.ParseFloat(fields[0], 64)
	if err != nil {
		return 0, false
	}
	return time.Duration(secs * float64(time.Second)), true
}

// formatUptime renders a duration as the coarsest useful unit.
func formatUptime(d time.Duration) string {
	days := int(d.Hours()) / 24
	hours := int(d.Hours()) % 24
	mins := int(d.Minutes()) % 60
	if days > 0 {
		return fmt.Sprintf("%dd %dh", days, hours)
	}
	if hours > 0 {
		return fmt.Sprintf("%dh %dm", hours, mins)
	}
	return fmt.Sprintf("%dm", mins)
}

// diskUsedPct reports the used-space percentage for the filesystem at path.
func diskUsedPct(path string) (float64, bool) {
	var stat syscall.Statfs_t
	if err := syscall.Statfs(path, &stat); err != nil {
		return 0, false
	}
	total := float64(stat.Blocks) * float64(stat.Bsize)
	free := float64(stat.Bavail) * float64(stat.Bsize)
	if total == 0 {
		return 0, false
	}
	return (total - free) / total * 100, true
}

// serviceActive runs `systemctl is-active <name>` with a 3s timeout. Any
// failure (binary missing, as on macOS dev) reports "n/a" rather than error.
func serviceActive(ctx context.Context, name string) string {
	cctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	out, err := exec.CommandContext(cctx, "systemctl", "is-active", name).Output()
	state := strings.TrimSpace(string(out))
	if state == "" {
		if err != nil {
			return "n/a"
		}
		return "unknown"
	}
	return state
}

// checkSite does a 5s GET against the public site and wants a 200.
func checkSite(ctx context.Context) bool {
	cctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(cctx, http.MethodGet, "https://noxioai.com", nil)
	if err != nil {
		return false
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	return resp.StatusCode == http.StatusOK
}

// evaluateProblems checks a SystemStatus against fixed alert thresholds and
// returns a sorted list of human-readable problems (empty = all clear).
// Unavailable numeric fields (-1) never trigger a threshold — fail-soft.
func evaluateProblems(s SystemStatus) []string {
	var problems []string
	if s.DiskPct >= 0 && s.DiskPct > 85 {
		problems = append(problems, fmt.Sprintf("disk %.0f%% > 85%%", s.DiskPct))
	}
	if s.MemPct >= 0 && s.MemPct > 92 {
		problems = append(problems, fmt.Sprintf("mem %.0f%% > 92%%", s.MemPct))
	}
	if s.Load1 >= 0 && s.NumCPU > 0 && s.Load1 > 2*float64(s.NumCPU) {
		problems = append(problems, fmt.Sprintf("load1 %.2f > 2x%d cores", s.Load1, s.NumCPU))
	}
	for _, svc := range s.Services {
		if svc.State != "active" {
			problems = append(problems, fmt.Sprintf("service %s: %s", svc.Name, svc.State))
		}
	}
	if !s.DBOnline {
		problems = append(problems, "db: down")
	}
	if !s.SiteOK {
		problems = append(problems, "site: not 200")
	}
	sort.Strings(problems)
	return problems
}

// problemsHash fingerprints a problem set so healthcheck can detect change.
func problemsHash(problems []string) string {
	sum := sha256.Sum256([]byte(strings.Join(problems, "\n")))
	return hex.EncodeToString(sum[:])
}

// hasStateChanged compares a freshly evaluated problem set against the
// last-known hash. Pure — the caller supplies oldHash (read from disk).
func hasStateChanged(oldHash string, problems []string) (changed bool, newHash string) {
	newHash = problemsHash(problems)
	return newHash != oldHash, newHash
}

// readLastHealthHash reads the persisted problem-set hash. Missing file
// (fresh install, or local dev without /var/lib/jarvis) reads as "".
func readLastHealthHash() string {
	b, err := os.ReadFile(healthStatePath)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(b))
}

// writeLastHealthHash persists the new hash. Fails soft — a write error
// (e.g. no /var/lib/jarvis locally) just means the next run re-alerts.
func writeLastHealthHash(hash string) {
	_ = os.WriteFile(healthStatePath, []byte(hash), 0o644)
}

func formatPct(v float64) string {
	if v < 0 {
		return "n/a"
	}
	return fmt.Sprintf("%.1f%%", v)
}

func formatLoad(s SystemStatus) string {
	if s.Load1 < 0 {
		return "n/a"
	}
	return fmt.Sprintf("%.2f %.2f %.2f", s.Load1, s.Load5, s.Load15)
}

func boolStr(b bool, t, f string) string {
	if b {
		return t
	}
	return f
}

// renderHealth is the one-screen plain-text status for `jarvis health`.
func renderHealth(s SystemStatus) string {
	var b strings.Builder
	fmt.Fprintf(&b, "JARVIS health — %s\n", s.Hostname)
	fmt.Fprintf(&b, "uptime: %s\n", s.Uptime)
	fmt.Fprintf(&b, "load:   %s\n", formatLoad(s))
	fmt.Fprintf(&b, "mem:    %s\n", formatPct(s.MemPct))
	fmt.Fprintf(&b, "disk:   %s\n", formatPct(s.DiskPct))
	b.WriteString("services:\n")
	for _, svc := range s.Services {
		fmt.Fprintf(&b, "  %-16s %s\n", svc.Name, svc.State)
	}
	fmt.Fprintf(&b, "db:     %s\n", boolStr(s.DBOnline, "online", "offline"))
	fmt.Fprintf(&b, "site:   %s\n", boolStr(s.SiteOK, "200", "down"))
	if problems := evaluateProblems(s); len(problems) > 0 {
		b.WriteString("\nPROBLEMS:\n")
		for _, p := range problems {
			fmt.Fprintf(&b, "  - %s\n", p)
		}
	} else {
		b.WriteString("\nall clear\n")
	}
	return b.String()
}

// serverBriefSection summarizes host health for the morning brief. Best
// effort — collectSystemStatus never errors, so this never blocks RunBrief.
func serverBriefSection(ctx context.Context, db *sql.DB) string {
	s := collectSystemStatus(ctx, db)
	up := 0
	for _, svc := range s.Services {
		if svc.State == "active" {
			up++
		}
	}
	return fmt.Sprintf("\n🖥️ SERVER: load %s · mem %s · disk %s · services %d/%d up\n",
		formatLoad(s), formatPct(s.MemPct), formatPct(s.DiskPct), up, len(s.Services))
}
