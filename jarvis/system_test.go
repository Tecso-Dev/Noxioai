package main

import (
	"fmt"
	"testing"
)

func TestParseMemInfo(t *testing.T) {
	fixture := `MemTotal:        8000000 kB
MemFree:          500000 kB
MemAvailable:    2000000 kB
Buffers:          100000 kB`
	pct, ok := parseMemInfo(fixture)
	if !ok {
		t.Fatal("expected ok")
	}
	want := (8000000.0 - 2000000.0) / 8000000.0 * 100
	if pct != want {
		t.Errorf("got %.4f want %.4f", pct, want)
	}
}

func TestParseMemInfoMissingFields(t *testing.T) {
	if _, ok := parseMemInfo("garbage\nnot meminfo"); ok {
		t.Fatal("expected not ok on missing fields")
	}
}

func TestParseLoadAvg(t *testing.T) {
	l1, l5, l15, ok := parseLoadAvg("0.52 0.35 0.20 1/234 5678")
	if !ok {
		t.Fatal("expected ok")
	}
	if l1 != 0.52 || l5 != 0.35 || l15 != 0.20 {
		t.Errorf("got %v %v %v", l1, l5, l15)
	}
}

func TestParseLoadAvgMalformed(t *testing.T) {
	if _, _, _, ok := parseLoadAvg("nope"); ok {
		t.Fatal("expected not ok")
	}
}

func TestParseUptimeSeconds(t *testing.T) {
	d, ok := parseUptimeSeconds("362980.15 100.00")
	if !ok {
		t.Fatal("expected ok")
	}
	if d.Seconds() != 362980.15 {
		t.Errorf("got %v", d.Seconds())
	}
}

func TestFormatUptime(t *testing.T) {
	cases := []struct {
		secs float64
		want string
	}{
		{5 * 60, "5m"},
		{3*3600 + 20*60, "3h 20m"},
		{2*86400 + 4*3600, "2d 4h"},
	}
	for _, c := range cases {
		d, _ := parseUptimeSeconds(fmtSecs(c.secs))
		if got := formatUptime(d); got != c.want {
			t.Errorf("formatUptime(%v) = %q want %q", c.secs, got, c.want)
		}
	}
}

func fmtSecs(s float64) string {
	return fmt.Sprintf("%f 100.00", s)
}

func TestEvaluateProblemsAllClear(t *testing.T) {
	s := SystemStatus{
		Load1: 0.1, MemPct: 10, DiskPct: 10, NumCPU: 4,
		Services: []ServiceState{{Name: "jarvis", State: "active"}},
		DBOnline: true, SiteOK: true,
	}
	if got := evaluateProblems(s); len(got) != 0 {
		t.Errorf("expected no problems, got %v", got)
	}
}

func TestEvaluateProblemsEachThreshold(t *testing.T) {
	base := func() SystemStatus {
		return SystemStatus{
			Load1: 0.1, MemPct: 10, DiskPct: 10, NumCPU: 4,
			Services: []ServiceState{{Name: "jarvis", State: "active"}},
			DBOnline: true, SiteOK: true,
		}
	}

	s := base()
	s.DiskPct = 90
	if got := evaluateProblems(s); len(got) != 1 {
		t.Errorf("disk: expected 1 problem, got %v", got)
	}

	s = base()
	s.MemPct = 95
	if got := evaluateProblems(s); len(got) != 1 {
		t.Errorf("mem: expected 1 problem, got %v", got)
	}

	s = base()
	s.Load1 = 9 // > 2*4
	if got := evaluateProblems(s); len(got) != 1 {
		t.Errorf("load: expected 1 problem, got %v", got)
	}

	s = base()
	s.Services = []ServiceState{{Name: "caddy", State: "failed"}}
	if got := evaluateProblems(s); len(got) != 1 {
		t.Errorf("service: expected 1 problem, got %v", got)
	}

	s = base()
	s.DBOnline = false
	if got := evaluateProblems(s); len(got) != 1 {
		t.Errorf("db: expected 1 problem, got %v", got)
	}

	s = base()
	s.SiteOK = false
	if got := evaluateProblems(s); len(got) != 1 {
		t.Errorf("site: expected 1 problem, got %v", got)
	}
}

func TestEvaluateProblemsCombination(t *testing.T) {
	s := SystemStatus{
		Load1: 20, MemPct: 99, DiskPct: 99, NumCPU: 2,
		Services: []ServiceState{
			{Name: "jarvis", State: "active"},
			{Name: "caddy", State: "failed"},
			{Name: "postgresql", State: "inactive"},
		},
		DBOnline: false, SiteOK: false,
	}
	got := evaluateProblems(s)
	if len(got) != 7 { // disk, mem, load, 2 services, db, site
		t.Errorf("expected 7 problems, got %d: %v", len(got), got)
	}
}

func TestEvaluateProblemsUnavailableFieldsDoNotAlert(t *testing.T) {
	s := SystemStatus{
		Load1: -1, MemPct: -1, DiskPct: -1, NumCPU: 4,
		Services: []ServiceState{{Name: "jarvis", State: "active"}},
		DBOnline: true, SiteOK: true,
	}
	if got := evaluateProblems(s); len(got) != 0 {
		t.Errorf("expected n/a fields to fail soft (no alert), got %v", got)
	}
}

func TestHasStateChanged(t *testing.T) {
	problemsA := []string{"disk 90% > 85%"}
	problemsB := []string{"mem 99% > 92%"}

	changed, hashA := hasStateChanged("", problemsA)
	if !changed {
		t.Fatal("expected change from empty state")
	}

	changed, hashA2 := hasStateChanged(hashA, problemsA)
	if changed {
		t.Errorf("expected no change for identical problem set, got new hash %q vs %q", hashA2, hashA)
	}

	changed, hashB := hasStateChanged(hashA, problemsB)
	if !changed {
		t.Fatal("expected change when problem set differs")
	}
	if hashA == hashB {
		t.Error("expected different hashes for different problem sets")
	}

	changed, _ = hasStateChanged(hashB, nil)
	if !changed {
		t.Fatal("expected change when problems clear to none")
	}
}

func TestFormatOpenRouterCredits(t *testing.T) {
	fixture := []byte(`{"data":{"total_credits":25.5,"total_usage":3.25}}`)
	got, ok := formatOpenRouterCredits(fixture)
	if !ok {
		t.Fatal("expected ok")
	}
	if got != "$22.25" {
		t.Errorf("got %q want $22.25", got)
	}
}

func TestFormatOpenRouterCreditsMalformed(t *testing.T) {
	if _, ok := formatOpenRouterCredits([]byte("not json")); ok {
		t.Fatal("expected not ok on malformed JSON")
	}
}

func TestProblemsHashOrderIndependent(t *testing.T) {
	// evaluateProblems always returns a sorted slice, so the hash of two
	// differently-ordered-but-equal sets must match once sorted upstream.
	a := problemsHash([]string{"x", "y"})
	b := problemsHash([]string{"x", "y"})
	if a != b {
		t.Error("expected identical hash for identical input")
	}
	c := problemsHash([]string{"y", "x"})
	if a == c {
		t.Error("expected different hash for different order (caller must sort first)")
	}
}
