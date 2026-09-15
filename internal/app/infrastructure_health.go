package app

import (
	"fmt"
	"strconv"
	"strings"
)

type InfrastructureHealthReport struct {
	Status   string                       `json:"status"`
	Findings []InfrastructureHealthFinding `json:"findings"`
}

type InfrastructureHealthFinding struct {
	Severity string `json:"severity"`
	Check    string `json:"check"`
	Message  string `json:"message"`
}

const (
	healthSeverityWarning  = "warning"
	healthSeverityCritical = "critical"
)

func analyzeInfrastructureHealth(checks []aiDiagnosticCheck) InfrastructureHealthReport {
	report := InfrastructureHealthReport{Status: "healthy", Findings: []InfrastructureHealthFinding{}}
	for _, check := range checks {
		if check.Result.Error != "" || check.Result.ExitCode != 0 {
			report.Findings = append(report.Findings, InfrastructureHealthFinding{
				Severity: healthSeverityWarning,
				Check:    check.Name,
				Message:  fmt.Sprintf("Diagnostic check failed: %s", firstNonEmpty(check.Result.Error, strings.TrimSpace(check.Result.Stderr))),
			})
			continue
		}

		switch check.Name {
		case "memory":
			analyzeMemory(check.Result.Stdout, &report)
		case "disk":
			analyzeDisk(check.Result.Stdout, &report)
		}
	}

	for _, finding := range report.Findings {
		if finding.Severity == healthSeverityCritical {
			report.Status = "critical"
			return report
		}
		if finding.Severity == healthSeverityWarning && report.Status == "healthy" {
			report.Status = "degraded"
		}
	}
	return report
}

func analyzeMemory(output string, report *InfrastructureHealthReport) {
	for _, line := range strings.Split(output, "\n") {
		fields := strings.Fields(line)
		if len(fields) < 7 || fields[0] != "Mem:" {
			continue
		}
		total, totalOK := parseHumanBytes(fields[1])
		available, availableOK := parseHumanBytes(fields[6])
		if !totalOK || !availableOK || total <= 0 {
			report.Findings = append(report.Findings, InfrastructureHealthFinding{
				Severity: healthSeverityWarning,
				Check:    "memory",
				Message:  "Memory output could not be parsed reliably.",
			})
			return
		}
		availablePercent := available / total * 100
		if availablePercent < 10 {
			report.Findings = append(report.Findings, InfrastructureHealthFinding{
				Severity: healthSeverityCritical,
				Check:    "memory",
				Message:  fmt.Sprintf("Available memory is %.1f%% of total memory.", availablePercent),
			})
		} else if availablePercent < 20 {
			report.Findings = append(report.Findings, InfrastructureHealthFinding{
				Severity: healthSeverityWarning,
				Check:    "memory",
				Message:  fmt.Sprintf("Available memory is %.1f%% of total memory.", availablePercent),
			})
		}
		return
	}
}

func analyzeDisk(output string, report *InfrastructureHealthReport) {
	for _, line := range strings.Split(output, "\n") {
		fields := strings.Fields(line)
		if len(fields) < 6 || !strings.HasSuffix(fields[4], "%") {
			continue
		}
		percent, err := strconv.Atoi(strings.TrimSuffix(fields[4], "%"))
		if err != nil || percent < 0 || percent > 100 {
			continue
		}
		mount := fields[5]
		severity := ""
		if percent > 95 {
			severity = healthSeverityCritical
		} else if percent > 90 {
			severity = healthSeverityWarning
		}
		if severity != "" {
			report.Findings = append(report.Findings, InfrastructureHealthFinding{
				Severity: severity,
				Check:    "disk",
				Message:  fmt.Sprintf("Filesystem %s is %d%% full.", mount, percent),
			})
		}
	}
}

func parseHumanBytes(value string) (float64, bool) {
	value = strings.TrimSpace(strings.ToUpper(value))
	if value == "" {
		return 0, false
	}
	multiplier := float64(1)
	suffixes := []struct {
		suffix string
		factor float64
	}{
		{"PI", 1 << 50}, {"TI", 1 << 40}, {"GI", 1 << 30}, {"MI", 1 << 20}, {"KI", 1 << 10},
		{"P", 1e15}, {"T", 1e12}, {"G", 1e9}, {"M", 1e6}, {"K", 1e3},
	}
	for _, item := range suffixes {
		if strings.HasSuffix(value, item.suffix) {
			value = strings.TrimSpace(strings.TrimSuffix(value, item.suffix))
			multiplier = item.factor
			break
		}
	}
	number, err := strconv.ParseFloat(value, 64)
	if err != nil {
		return 0, false
	}
	return number * multiplier, true
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return "unknown error"
}
