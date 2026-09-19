package app

import (
	"regexp"
	"strings"
	"testing"

	"eiksy/internal/domain/ai"
	"eiksy/internal/storage/disk"
	"eiksy/internal/storage/memory"
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

func TestRedactAuditValueRedactsDelimitedAndCamelCaseSecrets(t *testing.T) {
	input := `token: "secret-token" apiKey: "secret-api-key" MY_AUTHORIZATION="secret-auth"`
	got := redactAuditValue(input)

	for _, secret := range []string{"secret-token", "secret-api-key", "secret-auth"} {
		if strings.Contains(got, secret) {
			t.Fatalf("redacted audit value contains secret %q: %q", secret, got)
		}
	}
	if got == input {
		t.Fatalf("expected sensitive values to be redacted")
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

func TestRecordCommandAuditKeepsBoundedInMemoryTrail(t *testing.T) {
	service := NewService(memory.NewStore(), nil, nil)
	for i := 0; i < maxCommandAuditEvents+25; i++ {
		service.recordCommandAudit("", "session", "hostname", "allow", "not_required", "success", 0, 1, ai.CommandAuditEvent{})
	}
	trail := service.GetCommandAuditTrail()
	if len(trail) != maxCommandAuditEvents {
		t.Fatalf("expected %d audit events, got %d", maxCommandAuditEvents, len(trail))
	}
}

func TestRecordCommandAuditRedactsBeforePersistence(t *testing.T) {
	service := NewService(memory.NewStore(), nil, nil)
	service.recordCommandAudit("", "session", "curl --token=super-secret", "ask", "denied", "policy_denied", 1, 2, ai.CommandAuditEvent{Error: "password=hunter2"})
	event := service.GetCommandAuditTrail()[0]
	if strings.Contains(event.Command, "super-secret") || strings.Contains(event.Error, "hunter2") {
		t.Fatalf("audit event leaked secret: %+v", event)
	}
}

func TestRecordCommandAuditPersistsOnlyRedactedValues(t *testing.T) {
	dir := t.TempDir()
	store, err := disk.NewStoreAt(dir)
	if err != nil {
		t.Fatalf("create disk store: %v", err)
	}

	service := NewService(store, nil, nil)
	service.recordCommandAudit(
		"",
		"session",
		`curl --token=super-secret --header "X-Api-Key: another-secret"`,
		"ask",
		"denied",
		"policy_denied",
		1,
		2,
		ai.CommandAuditEvent{Error: "password=hunter2"},
	)

	reloaded, err := disk.NewStoreAt(dir)
	if err != nil {
		t.Fatalf("reload disk store: %v", err)
	}
	trail := NewService(reloaded, nil, nil).GetCommandAuditTrail()
	if len(trail) != 1 {
		t.Fatalf("expected one persisted audit event, got %d", len(trail))
	}
	event := trail[0]
	for _, secret := range []string{"super-secret", "another-secret", "hunter2"} {
		if strings.Contains(event.Command, secret) || strings.Contains(event.Error, secret) {
			t.Fatalf("persisted audit event leaked secret %q: %+v", secret, event)
		}
	}
}

func TestRedactAuditValuePreservesSafeStatus(t *testing.T) {
	for _, value := range []string{"executed", "execution_failed", "approval_required", "policy_denied"} {
		if got := redactAuditValue(value); got != value {
			t.Fatalf("safe status changed: %q -> %q", value, got)
		}
	}
}

var _ = regexp.MustCompile
