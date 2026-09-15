package app

import (
	"testing"

	"eiksy/internal/domain/sessions"
	"eiksy/internal/domain/workspace"
	"eiksy/internal/storage/memory"
)

func TestGetAIInfrastructureContextExcludesSecrets(t *testing.T) {
	store := memory.NewStore()
	if err := store.UpsertSessionProfile(sessions.Profile{
		ID:            "profile-1",
		Name:          "production",
		Group:         "prod",
		Tags:          []string{"critical", "linux"},
		ProtocolID:    "ssh",
		Host:          "10.0.0.10",
		Port:          22,
		Username:      "operator",
		Password:      "super-secret",
		KeyPassphrase: "key-secret",
	}); err != nil {
		t.Fatalf("seed profile: %v", err)
	}
	store.OpenRuntimeTab(workspace.Tab{
		ID:          "session-1",
		Title:       "production",
		ProtocolID:  "ssh",
		ProfileID:   "profile-1",
		Status:      "connected",
		Description: "production host",
	})

	service := NewService(store, &nativeTestSSHManager{}, nil)
	context, err := service.GetAIInfrastructureContext("session-1")
	if err != nil {
		t.Fatalf("get infrastructure context: %v", err)
	}
	if context.ActiveSession == nil {
		t.Fatal("expected active session context")
	}

	got := context.ActiveSession
	if got.Host != "10.0.0.10" || got.Username != "operator" || got.CurrentDir != "." {
		t.Fatalf("unexpected safe context: %+v", got)
	}
	if got.Group != "prod" || len(got.Tags) != 2 {
		t.Fatalf("expected profile metadata, got %+v", got)
	}
	if got.Name != "production" || got.Status != "connected" {
		t.Fatalf("unexpected session metadata: %+v", got)
	}

	serialized := got.Host + got.Username + got.Name + got.Description
	if containsAny(serialized, "super-secret", "key-secret") {
		t.Fatal("credential material leaked into infrastructure context")
	}
}

func TestGetAIInfrastructureContextMissingSession(t *testing.T) {
	service := NewService(memory.NewStore(), &nativeTestSSHManager{}, nil)
	if _, err := service.GetAIInfrastructureContext("missing"); err == nil {
		t.Fatal("expected missing session error")
	}
}

func containsAny(value string, needles ...string) bool {
	for _, needle := range needles {
		if needle != "" && contains(value, needle) {
			return true
		}
	}
	return false
}

func contains(value, needle string) bool {
	for i := 0; i+len(needle) <= len(value); i++ {
		if value[i:i+len(needle)] == needle {
			return true
		}
	}
	return false
}
