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

func TestAIInfrastructureContextNeverContainsCredentialMaterial(t *testing.T) {
	store := memory.NewStore()
	if err := store.UpsertSessionProfile(sessions.Profile{
		ID:            "profile-secret-boundary",
		Name:          "production",
		Group:         "prod",
		ProtocolID:    "ssh",
		Host:          "10.0.0.10",
		Port:          22,
		Username:      "operator",
		Password:      "super-secret-password",
		KeyPassphrase: "super-secret-key-passphrase",
	}); err != nil {
		t.Fatalf("seed profile: %v", err)
	}
	store.OpenRuntimeTab(workspace.Tab{
		ID:          "session-secret-boundary",
		Title:       "production",
		ProtocolID:  "ssh",
		ProfileID:   "profile-secret-boundary",
		Status:      "connected",
		Description: "production host",
	})

	service := NewService(store, &nativeTestSSHManager{}, nil)
	context, err := service.GetAIInfrastructureContext("session-secret-boundary")
	if err != nil {
		t.Fatalf("get infrastructure context: %v", err)
	}
	serialized, err := json.Marshal(context)
	if err != nil {
		t.Fatalf("marshal infrastructure context: %v", err)
	}

	payload := string(serialized)
	for _, secret := range []string{"super-secret-password", "super-secret-key-passphrase"} {
		if strings.Contains(payload, secret) {
			t.Fatalf("AI infrastructure context leaked credential material %q: %s", secret, payload)
		}
	}
}

func TestCommandPolicyResolutionAuditDoesNotLeakCredentialMaterial(t *testing.T) {
	service := NewService(memory.NewStore(), nil, nil)
	request := ai.CommandRequest{
		ID:        "cmdreq-secret-boundary",
		ToolID:    "shell",
		SessionID: "session-1",
		Command:   `curl -H "Authorization: Bearer super-secret-token" https://example.test`,
	}

	service.RecordCommandPolicyResolutionForApp(
		request,
		ai.CommandPermissionModeNow,
		&secretBoundaryError{message: "ssh failed with password=super-secret-password"},
	)

	trail := service.GetCommandAuditTrail()
	if len(trail) != 1 {
		t.Fatalf("expected one audit event, got %d", len(trail))
	}
	serialized, err := json.Marshal(trail[0])
	if err != nil {
		t.Fatalf("marshal audit event: %v", err)
	}
	payload := string(serialized)
	for _, secret := range []string{"super-secret-token", "super-secret-password"} {
		if strings.Contains(payload, secret) {
			t.Fatalf("policy-resolution audit leaked secret %q: %s", secret, payload)
		}
	}
}

type secretBoundaryError struct {
	message string
}

func (e *secretBoundaryError) Error() string { return e.message }
