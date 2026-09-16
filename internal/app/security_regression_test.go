package app

import (
	"encoding/json"
	"strings"
	"testing"

	"eiksy/internal/domain/ai"
	"eiksy/internal/domain/sessions"
	"eiksy/internal/domain/workspace"
	"eiksy/internal/storage/memory"
)

func TestSecurityRegressionShellStateContainsNoCredentialMaterial(t *testing.T) {
	store := memory.NewStore()
	if err := store.UpsertSessionProfile(sessions.Profile{
		ID:            "security-regression-profile",
		Name:          "production",
		ProtocolID:    "ssh",
		Host:          "prod.example",
		Port:          22,
		Username:      "operator",
		Password:      sessions.EncryptedString("regression-password"),
		KeyPassphrase: sessions.EncryptedString("regression-key-passphrase"),
	}); err != nil {
		t.Fatalf("seed profile: %v", err)
	}
	store.OpenRuntimeTab(workspace.Tab{
		ID:         "security-regression-session",
		Title:      "production",
		ProtocolID: "ssh",
		ProfileID:  "security-regression-profile",
		Status:     "connected",
	})
	store.UpdateAIState(ai.WorkspaceState{
		Providers: []ai.ProviderDescriptor{{
			ID:         "provider",
			Name:       "provider",
			Endpoint:   "https://example.test",
			Token:      "regression-provider-token",
			HasToken:   true,
			Configured: true,
		}},
	})

	service := NewService(store, nil, nil)
	payload, err := json.Marshal(service.GetShellState())
	if err != nil {
		t.Fatalf("marshal shell state: %v", err)
	}
	assertSecurityRegressionDoesNotContain(t, string(payload),
		"regression-password",
		"regression-key-passphrase",
		"regression-provider-token",
	)
}

func TestSecurityRegressionAIContextExcludesCredentialMaterial(t *testing.T) {
	store := memory.NewStore()
	if err := store.UpsertSessionProfile(sessions.Profile{
		ID:            "security-regression-ai-profile",
		Name:          "production",
		ProtocolID:    "ssh",
		Host:          "prod.example",
		Port:          22,
		Username:      "operator",
		Password:      sessions.EncryptedString("ai-password-secret"),
		KeyPassphrase: sessions.EncryptedString("ai-key-secret"),
	}); err != nil {
		t.Fatalf("seed profile: %v", err)
	}
	store.OpenRuntimeTab(workspace.Tab{
		ID:         "security-regression-ai-session",
		Title:      "production",
		ProtocolID: "ssh",
		ProfileID:  "security-regression-ai-profile",
		Status:     "connected",
	})

	service := NewService(store, &nativeTestSSHManager{}, nil)
	context, err := service.GetAIInfrastructureContext("security-regression-ai-session")
	if err != nil {
		t.Fatalf("get AI infrastructure context: %v", err)
	}
	payload, err := json.Marshal(context)
	if err != nil {
		t.Fatalf("marshal AI context: %v", err)
	}
	assertSecurityRegressionDoesNotContain(t, string(payload),
		"ai-password-secret",
		"ai-key-secret",
	)
}

func TestSecurityRegressionAuditRedactsCommandErrorAndResult(t *testing.T) {
	service := NewService(memory.NewStore(), nil, nil)
	request := ai.CommandRequest{
		ID:        "security-regression-command",
		ToolID:    "shell",
		SessionID: "security-regression-session",
		Command:   `curl -H "Authorization: Bearer command-secret" https://example.test`,
	}

	service.RecordCommandAudit(request, "output contains result-secret", &secretBoundaryError{message: "password=error-secret token=error-token"})

	trail := service.GetCommandAuditTrail()
	if len(trail) != 1 {
		t.Fatalf("expected one audit event, got %d", len(trail))
	}
	payload, err := json.Marshal(trail[0])
	if err != nil {
		t.Fatalf("marshal audit event: %v", err)
	}
	assertSecurityRegressionDoesNotContain(t, string(payload),
		"command-secret",
		"result-secret",
		"error-secret",
		"error-token",
	)
}

func TestSecurityRegressionDisabledCommandCannotExecuteThroughPolicy(t *testing.T) {
	policy := ai.CommandPolicy{
		Tools: []ai.CommandTool{{ID: "shell", Enabled: false}},
		CommandRules: []ai.CommandRule{{
			Pattern: "echo *",
			Action:  ai.CommandPermissionAllow,
		}},
	}

	got, reason := evaluateCommandPolicy(policy, "shell", "security-regression-session", "echo hello")
	if got != commandPolicyDecisionDeny {
		t.Fatalf("disabled tool must be denied, got %q (%s)", got, reason)
	}
}

func assertSecurityRegressionDoesNotContain(t *testing.T, payload string, secrets ...string) {
	t.Helper()
	for _, secret := range secrets {
		if strings.Contains(payload, secret) {
			t.Fatalf("security boundary leaked secret %q in payload: %s", secret, payload)
		}
	}
}
