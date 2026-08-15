package app

import (
	"context"
	"testing"

	"opsy/internal/storage/memory"
)

type fakeLocalModelManager struct {
	downloadPath string
	endpoint     string
	command      string
	downloadErr  error
	startErr     error
}

func (f fakeLocalModelManager) DownloadQwen3Model(context.Context) (string, error) {
	return f.downloadPath, f.downloadErr
}

func (f fakeLocalModelManager) StartLocalServer(context.Context, string) (string, string, error) {
	return f.endpoint, f.command, f.startErr
}

func TestGetShellStateIncludesScaffoldedDomains(t *testing.T) {
	service := NewService(memory.NewStore(), nil)

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
	service := NewService(memory.NewStore(), nil)

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
	service := NewService(memory.NewStore(), nil)

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
	service := NewService(memory.NewStore(), nil)

	if err := service.SelectAIProvider("openai-compatible-cloud"); err != nil {
		t.Fatalf("select ai provider: %v", err)
	}

	state := service.GetShellState()
	if !state.AI.Providers[1].Selected {
		t.Fatal("expected cloud provider to be selected")
	}
	if state.AI.Providers[0].Selected {
		t.Fatal("expected local provider to be deselected")
	}
}

func TestSaveCloudProviderStoresEndpointAndConfiguration(t *testing.T) {
	service := NewService(memory.NewStore(), nil)

	if err := service.SaveCloudProvider("https://models.example.com/v1", "secret-token"); err != nil {
		t.Fatalf("save cloud provider: %v", err)
	}

	state := service.GetShellState()
	cloudProvider := state.AI.Providers[1]
	if !cloudProvider.Selected {
		t.Fatal("expected cloud provider to be selected")
	}
	if !cloudProvider.Configured {
		t.Fatal("expected cloud provider to be configured")
	}
	if cloudProvider.Endpoint != "https://models.example.com/v1" {
		t.Fatalf("expected cloud endpoint to be saved, got %q", cloudProvider.Endpoint)
	}
	if cloudProvider.Status != "ready" {
		t.Fatalf("expected cloud provider status ready, got %q", cloudProvider.Status)
	}
}

func TestDownloadAndStartLocalModelUpdatesProviderState(t *testing.T) {
	service := NewService(memory.NewStore(), fakeLocalModelManager{
		downloadPath: "/tmp/qwen3.gguf",
		endpoint:     "http://127.0.0.1:8012/v1",
		command:      "llama-server -m /tmp/qwen3.gguf --host 127.0.0.1 --port 8012",
	})

	if err := service.DownloadLocalModel(context.Background()); err != nil {
		t.Fatalf("download local model: %v", err)
	}
	if err := service.StartLocalModel(context.Background()); err != nil {
		t.Fatalf("start local model: %v", err)
	}

	state := service.GetShellState()
	localProvider := state.AI.Providers[0]
	if !localProvider.Selected {
		t.Fatal("expected local provider to be selected")
	}
	if !localProvider.Configured {
		t.Fatal("expected local provider to be configured")
	}
	if localProvider.LocalPath != "/tmp/qwen3.gguf" {
		t.Fatalf("expected local model path to be saved, got %q", localProvider.LocalPath)
	}
	if localProvider.Endpoint != "http://127.0.0.1:8012/v1" {
		t.Fatalf("expected local endpoint to be saved, got %q", localProvider.Endpoint)
	}
	if localProvider.Status != "running via llama.cpp" {
		t.Fatalf("expected local provider status to reflect llama.cpp, got %q", localProvider.Status)
	}
}
