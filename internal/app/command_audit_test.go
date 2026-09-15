package app

import (
	"strings"
	"testing"
)

func TestRedactAuditValueRedactsSensitiveText(t *testing.T) {
	input := "request failed password=hunter2 token=abc123 authorization: Bearer super-secret"
	got := redactAuditValue(input)

	for _, secret := range []string{"hunter2", "abc123", "super-secret"} {
		if strings.Contains(got, secret) {
			t.Fatalf("redacted audit value contains secret %q: %q", secret, got)
		}
	}
	if !strings.Contains(got, "[REDACTED]") {
		t.Fatalf("expected redaction marker, got %q", got)
	}
}

func TestRedactAuditValueNormalizesAndBoundsText(t *testing.T) {
	input := strings.Repeat("x", maxCommandAuditText+100) + "\nsecret"
	got := redactAuditValue(input)

	if len(got) != maxCommandAuditText+len("...[TRUNCATED]") {
		t.Fatalf("unexpected bounded length: got %d", len(got))
	}
	if strings.ContainsAny(got, "\r\n\t") {
		t.Fatalf("audit value contains control whitespace: %q", got)
	}
	if !strings.HasSuffix(got, "...[TRUNCATED]") {
		t.Fatalf("missing truncation marker: %q", got[len(got)-20:])
	}
}

func TestRedactAuditValuePreservesSafeStatus(t *testing.T) {
	for _, value := range []string{"executed", "execution_failed", "approval_required", "policy_denied"} {
		if got := redactAuditValue(value); got != value {
			t.Fatalf("safe status changed: %q -> %q", value, got)
		}
	}
}
