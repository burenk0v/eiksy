package app

import (
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"

	"opsy/internal/domain/ai"
	"opsy/internal/domain/sessions"
	"opsy/internal/storage/memory"
)

func TestGetShellStateIncludesScaffoldedDomains(t *testing.T) {
	service := NewService(memory.NewStore(), nil, nil, nil)

	state := service.GetShellState()

	if len(state.Protocols) != 3 {
		t.Fatalf("expected 3 protocols, got %d", len(state.Protocols))
	}
	if state.SessionProfiles == nil {
		t.Fatal("expected session profiles slice")
	}
	if len(state.CredentialProviders) != 0 {
		t.Fatalf("expected 0 credential providers, got %d", len(state.CredentialProviders))
	}
	if len(state.AI.Providers) != 1 {
		t.Fatalf("expected 1 ai provider, got %d", len(state.AI.Providers))
	}
}

func TestLaunchSessionCreatesRuntimeTabAndHistory(t *testing.T) {
	service := NewService(seedStore(t), nil, nil, nil)

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

	var launchedProfileFound bool
	for _, profile := range after.SessionProfiles {
		if profile.ID == "artifact-mirror" {
			launchedProfileFound = true
			if profile.LastLaunchedAt == "" {
				t.Fatal("expected launched profile to record last launch time")
			}
			break
		}
	}
	if !launchedProfileFound {
		t.Fatal("expected launched profile to remain in shell state")
	}
}

func TestLaunchHistoryIsCapped(t *testing.T) {
	service := NewService(seedStore(t), nil, nil, nil)

	for range 150 {
		if _, err := service.LaunchSession("artifact-mirror"); err != nil {
			t.Fatalf("launch session: %v", err)
		}
	}

	state := service.GetShellState()
	if len(state.SessionHistory) != 100 {
		t.Fatalf("expected capped launch history of 100 entries, got %d", len(state.SessionHistory))
	}
}

func TestSelectAIProviderMarksCloudProviderSelected(t *testing.T) {
	service := NewService(memory.NewStore(), nil, nil, nil)

	if err := service.SelectAIProvider("openai-compatible-cloud"); err != nil {
		t.Fatalf("select ai provider: %v", err)
	}

	state := service.GetShellState()
	cloudProvider := mustFindProviderByID(t, state, "openai-compatible-cloud")
	if !cloudProvider.Selected {
		t.Fatal("expected cloud provider to be selected")
	}
}

func TestSaveCloudProviderStoresEndpointAndConfiguration(t *testing.T) {
	service := NewService(memory.NewStore(), nil, nil, nil)

	if err := service.SaveCloudProvider("gpt-5.6", "https://models.example.com/v1", "secret-token"); err != nil {
		t.Fatalf("save cloud provider: %v", err)
	}

	state := service.GetShellState()
	cloudProvider := mustFindProviderByID(t, state, "openai-compatible-cloud")
	if !cloudProvider.Selected {
		t.Fatal("expected cloud provider to be selected")
	}
	if !cloudProvider.Configured {
		t.Fatal("expected cloud provider to be configured")
	}
	if cloudProvider.Endpoint != "https://models.example.com/v1" {
		t.Fatalf("expected cloud endpoint to be saved, got %q", cloudProvider.Endpoint)
	}
	if cloudProvider.Model != "gpt-5.6" {
		t.Fatalf("expected cloud model to be saved, got %q", cloudProvider.Model)
	}
	if cloudProvider.Status != "ready" {
		t.Fatalf("expected cloud provider status ready, got %q", cloudProvider.Status)
	}
	if cloudProvider.Token != "secret-token" {
		t.Fatalf("expected cloud token to be saved, got %q", cloudProvider.Token)
	}
}

func TestSaveCloudProviderPreservesTokenWhenBlank(t *testing.T) {
	service := NewService(memory.NewStore(), nil, nil, nil)

	if err := service.SaveCloudProvider("gpt-5.6", "https://models.example.com/v1", "secret-token"); err != nil {
		t.Fatalf("save cloud provider: %v", err)
	}
	if err := service.SaveCloudProvider("gpt-5.7", "https://models.example.com/v2", ""); err != nil {
		t.Fatalf("save cloud provider with empty token: %v", err)
	}

	state := service.GetShellState()
	cloudProvider := mustFindProviderByID(t, state, "openai-compatible-cloud")
	if cloudProvider.Token != "secret-token" {
		t.Fatalf("expected cloud token to be preserved, got %q", cloudProvider.Token)
	}
	if cloudProvider.Model != "gpt-5.7" {
		t.Fatalf("expected updated model to be saved, got %q", cloudProvider.Model)
	}
	if cloudProvider.Endpoint != "https://models.example.com/v2" {
		t.Fatalf("expected updated endpoint to be saved, got %q", cloudProvider.Endpoint)
	}
}

func TestListCloudModelsFetchesAndSortsUniqueModels(t *testing.T) {
	var authHeader string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authHeader = r.Header.Get("Authorization")
		if r.URL.Path != "/v1/models" {
			t.Fatalf("expected /v1/models path, got %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":[{"id":"gpt-5.6"},{"id":"gpt-4.1"},{"id":"gpt-5.6"},{"id":" "}]}`))
	}))
	defer server.Close()

	service := NewService(memory.NewStore(), nil, nil, nil)
	models, err := service.ListCloudModels(server.URL+"/v1", "secret-token")
	if err != nil {
		t.Fatalf("list cloud models: %v", err)
	}
	if !slices.Equal(models, []string{"gpt-4.1", "gpt-5.6"}) {
		t.Fatalf("unexpected models list: %#v", models)
	}
	if !strings.HasPrefix(authHeader, "Bearer ") {
		t.Fatalf("expected bearer authorization header, got %q", authHeader)
	}
}

func TestListCloudModelsUsesSavedTokenWhenInputBlank(t *testing.T) {
	var authHeader string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authHeader = r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":[{"id":"gpt-5.6"}]}`))
	}))
	defer server.Close()

	service := NewService(memory.NewStore(), nil, nil, nil)
	if err := service.SaveCloudProvider("gpt-5.6", server.URL+"/v1", "saved-token"); err != nil {
		t.Fatalf("save cloud provider: %v", err)
	}

	if _, err := service.ListCloudModels(server.URL+"/v1", ""); err != nil {
		t.Fatalf("list cloud models: %v", err)
	}
	if !strings.HasPrefix(authHeader, "Bearer ") {
		t.Fatalf("expected bearer authorization header, got %q", authHeader)
	}
}

func seedStore(t *testing.T) *memory.Store {
	t.Helper()
	store := memory.NewStore()
	profiles := []sessions.Profile{
		{ID: "artifact-mirror", Name: "artifact-mirror", Group: "Shared Services", Tags: []string{"sftp", "artifacts"}, ProtocolID: "sftp", Host: "mirror.internal", Port: 22, Username: "mirrorbot"},
		{ID: "ops-linux-admin", Name: "ops-linux-admin", Group: "Production", Tags: []string{"linux", "ssh"}, ProtocolID: "ssh", Host: "prod-shell.internal", Port: 22, Username: "ops"},
	}
	for _, profile := range profiles {
		if err := store.UpsertSessionProfile(profile); err != nil {
			t.Fatalf("seed session profile: %v", err)
		}
	}
	return store
}

func mustFindProviderByID(t *testing.T, state ShellState, providerID string) ai.ProviderDescriptor {
	t.Helper()

	for _, provider := range state.AI.Providers {
		if provider.ID == providerID {
			return provider
		}
	}

	t.Fatalf("expected provider %q to exist", providerID)
	return ai.ProviderDescriptor{}
}
