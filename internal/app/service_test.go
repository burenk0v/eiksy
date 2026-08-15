package app

import (
	"testing"

	"opsy/internal/storage/memory"
)

func TestGetShellStateIncludesScaffoldedDomains(t *testing.T) {
	service := NewService(memory.NewStore())

	state := service.GetShellState()

	if len(state.Protocols) != 3 {
		t.Fatalf("expected 3 protocols, got %d", len(state.Protocols))
	}
	if len(state.SessionProfiles) == 0 {
		t.Fatal("expected session profiles")
	}
	if len(state.CredentialProviders) != 3 {
		t.Fatalf("expected 3 credential providers, got %d", len(state.CredentialProviders))
	}
	if len(state.AI.Providers) != 2 {
		t.Fatalf("expected 2 ai providers, got %d", len(state.AI.Providers))
	}
}

func TestLaunchSessionCreatesRuntimeTabAndHistory(t *testing.T) {
	service := NewService(memory.NewStore())

	before := service.GetShellState()
	launched, err := service.LaunchSession("artifact-mirror")
	if err != nil {
		t.Fatalf("launch session: %v", err)
	}
	after := service.GetShellState()

	if launched.ProfileID != "artifact-mirror" {
		t.Fatalf("expected profile artifact-mirror, got %s", launched.ProfileID)
	}
	if len(after.ActiveSessions) != len(before.ActiveSessions)+1 {
		t.Fatalf("expected active session count to grow, before=%d after=%d", len(before.ActiveSessions), len(after.ActiveSessions))
	}
	if len(after.SessionHistory) != len(before.SessionHistory)+1 {
		t.Fatalf("expected launch history count to grow, before=%d after=%d", len(before.SessionHistory), len(after.SessionHistory))
	}
	if len(after.SessionHistory) == 0 || after.SessionHistory[0].ProfileID != "artifact-mirror" {
		t.Fatalf("expected newest launch history entry to be artifact-mirror, got %+v", after.SessionHistory)
	}
}
