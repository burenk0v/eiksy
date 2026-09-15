package app

import (
	"testing"

	"eiksy/internal/domain/sessions"
)

func TestAnalyzeInfrastructureHealthHealthy(t *testing.T) {
	report := analyzeInfrastructureHealth([]aiDiagnosticCheck{
		{name: "memory", Result: sessions.CommandExecutionResult{Stdout: "              total        used        free      shared  buff/cache   available\nMem:           16Gi       8Gi       2Gi       100Mi       6Gi       10Gi\n"}},
		{name: "disk", Result: sessions.CommandExecutionResult{Stdout: "Filesystem      Size  Used Avail Use% Mounted on\n/dev/sda1       100G   50G   50G  50% /\n"}},
	})
	if report.Status != "healthy" {
		t.Fatalf("expected healthy status, got %q: %#v", report.Status, report)
	}
	if len(report.Findings) != 0 {
		t.Fatalf("expected no findings, got %#v", report.Findings)
	}
}

func TestAnalyzeInfrastructureHealthDiskThresholds(t *testing.T) {
	report := analyzeInfrastructureHealth([]aiDiagnosticCheck{
		{name: "disk", Result: sessions.CommandExecutionResult{Stdout: "Filesystem      Size  Used Avail Use% Mounted on\n/dev/sda1       100G   92G    8G  92% /\n/dev/sda2       100G   97G    3G  97% /data\n"}},
	})
	if report.Status != "critical" {
		t.Fatalf("expected critical status, got %q", report.Status)
	}
	if len(report.Findings) != 2 {
		t.Fatalf("expected two disk findings, got %#v", report.Findings)
	}
	if report.Findings[0].Severity != healthSeverityWarning {
		t.Fatalf("expected warning for 92%% disk usage, got %#v", report.Findings[0])
	}
	if report.Findings[1].Severity != healthSeverityCritical {
		t.Fatalf("expected critical for 97%% disk usage, got %#v", report.Findings[1])
	}
}

func TestAnalyzeInfrastructureHealthMemoryThresholds(t *testing.T) {
	report := analyzeInfrastructureHealth([]aiDiagnosticCheck{
		{name: "memory", Result: sessions.CommandExecutionResult{Stdout: "Mem: 16Gi 15Gi 100Mi 100Mi 900Mi 1Gi\n"}},
	})
	if report.Status != "critical" {
		t.Fatalf("expected critical status, got %q", report.Status)
	}
	if len(report.Findings) != 1 || report.Findings[0].Severity != healthSeverityCritical {
		t.Fatalf("expected one critical memory finding, got %#v", report.Findings)
	}
}

func TestAnalyzeInfrastructureHealthFailedCheck(t *testing.T) {
	report := analyzeInfrastructureHealth([]aiDiagnosticCheck{
		{name: "disk", Result: sessions.CommandExecutionResult{ExitCode: 1, Stderr: "df: permission denied", Error: "command failed"}},
	})
	if report.Status != "degraded" {
		t.Fatalf("expected degraded status, got %q", report.Status)
	}
	if len(report.Findings) != 1 || report.Findings[0].Severity != healthSeverityWarning {
		t.Fatalf("expected one warning finding, got %#v", report.Findings)
	}
}

func TestParseHumanBytes(t *testing.T) {
	cases := []struct {
		value string
		ok    bool
	}{
		{value: "16Gi", ok: true},
		{value: "512Mi", ok: true},
		{value: "1.5G", ok: true},
		{value: "invalid", ok: false},
	}
	for _, tc := range cases {
		_, ok := parseHumanBytes(tc.value)
		if ok != tc.ok {
			t.Fatalf("parseHumanBytes(%q) ok=%v, want %v", tc.value, ok, tc.ok)
		}
	}
}
