package app

import (
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"

	"opsy/internal/domain/ai"
	"opsy/internal/domain/sessions"
	"opsy/internal/domain/settings"
	"opsy/internal/securestorage"
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

func TestCreateSessionProfilePreservesPasswordWhenUpdatingWithoutPassword(t *testing.T) {
	store := memory.NewStore()
	original := sessions.Profile{
		ID:         "prod-ssh",
		Name:       "prod-ssh",
		ProtocolID: "ssh",
		Host:       "prod.internal",
		Port:       22,
		Username:   "ops",
		Password:   sessions.EncryptedString("keep-me"),
	}
	if err := store.UpsertSessionProfile(original); err != nil {
		t.Fatalf("seed session profile: %v", err)
	}
	service := NewService(store, nil, nil, nil)

	updated := original
	updated.Name = "prod-ssh-renamed"
	updated.Password = ""
	if err := service.CreateSessionProfile(updated); err != nil {
		t.Fatalf("update session profile: %v", err)
	}

	profile, ok := store.SessionProfile("prod-ssh")
	if !ok {
		t.Fatal("updated profile not found")
	}
	if !profile.HasPassword {
		t.Fatal("expected password flag to be preserved")
	}
	password, err := store.LoadSecret(securestorage.SessionPasswordKey("prod-ssh"))
	if err != nil {
		t.Fatalf("load preserved password: %v", err)
	}
	if password != "keep-me" {
		t.Fatalf("expected password to be preserved, got %q", password)
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
	if !cloudProvider.HasToken {
		t.Fatal("expected cloud provider token flag to be set")
	}
	token, err := service.store.LoadSecret(securestorage.AIProviderTokenKey("openai-compatible-cloud"))
	if err != nil {
		t.Fatalf("load saved cloud token: %v", err)
	}
	if token != "secret-token" {
		t.Fatalf("expected cloud token to be saved, got %q", token)
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
	token, err := service.store.LoadSecret(securestorage.AIProviderTokenKey("openai-compatible-cloud"))
	if err != nil {
		t.Fatalf("load preserved cloud token: %v", err)
	}
	if token != "secret-token" {
		t.Fatalf("expected cloud token to be preserved, got %q", token)
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

func TestListVaultSecretsRenewsTokenWhenEnabled(t *testing.T) {
	var renewCalls int
	var listCalls int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/auth/token/renew-self":
			renewCalls++
			if r.Method != http.MethodPost {
				t.Fatalf("expected renew POST, got %s", r.Method)
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"auth":{"renewable":true}}`))
		case "/v1/secret/metadata/team":
			listCalls++
			if got := r.URL.Query().Get("list"); got != "true" {
				t.Fatalf("expected list=true query, got %q", got)
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"data":{"keys":["prod/","db"]}}`))
		default:
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
	}))
	defer server.Close()

	store := memory.NewStore()
	cfg := store.Settings()
	cfg.VaultAddress = server.URL
	cfg.VaultMountPoint = "secret"
	cfg.VaultAutoRenewToken = true
	if err := store.StoreSecret(securestorage.VaultTokenKey(), "vault-token"); err != nil {
		t.Fatalf("store vault token: %v", err)
	}
	if err := store.UpdateSettings(cfg); err != nil {
		t.Fatalf("update settings: %v", err)
	}

	service := NewService(store, nil, nil, nil)
	entries, err := service.ListVaultSecrets("team")
	if err != nil {
		t.Fatalf("list vault secrets: %v", err)
	}
	if renewCalls != 1 {
		t.Fatalf("expected 1 renew call, got %d", renewCalls)
	}
	if listCalls != 1 {
		t.Fatalf("expected 1 list call, got %d", listCalls)
	}
	if len(entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(entries))
	}
}

func TestListVaultSecretsContinuesWhenRenewalFails(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/auth/token/renew-self":
			http.Error(w, "renew denied", http.StatusForbidden)
		case "/v1/secret/metadata/team":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"data":{"keys":["prod/"]}}`))
		default:
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
	}))
	defer server.Close()

	store := memory.NewStore()
	cfg := store.Settings()
	cfg.VaultAddress = server.URL
	cfg.VaultMountPoint = "secret"
	cfg.VaultAutoRenewToken = true
	if err := store.StoreSecret(securestorage.VaultTokenKey(), "vault-token"); err != nil {
		t.Fatalf("store vault token: %v", err)
	}
	if err := store.UpdateSettings(cfg); err != nil {
		t.Fatalf("update settings: %v", err)
	}

	service := NewService(store, nil, nil, nil)
	entries, err := service.ListVaultSecrets("team")
	if err != nil {
		t.Fatalf("list vault secrets: %v", err)
	}
	if len(entries) != 1 || entries[0].Name != "prod" {
		t.Fatalf("unexpected entries: %#v", entries)
	}
}

func TestClearChatKeepsMessageSliceUsable(t *testing.T) {
	service := NewService(memory.NewStore(), nil, nil, nil)

	if err := service.SendChatMessage(nil, "hello"); err == nil || !strings.Contains(err.Error(), "no AI provider is configured") {
		t.Fatalf("expected provider configuration error, got %v", err)
	}

	service.ClearChat()

	state := service.GetShellState()
	if state.AI.Messages == nil {
		t.Fatal("expected clear chat to leave an empty message slice")
	}
	if len(state.AI.Messages) != 0 {
		t.Fatalf("expected chat messages to be cleared, got %d", len(state.AI.Messages))
	}
	if state.AI.ChatSessionID == "" {
		t.Fatal("expected chat session id to be regenerated")
	}
}

func TestBuildPortForwardSpecsWithRemoteTarget(t *testing.T) {
	specs := buildPortForwardSpecs("8080,9000-9001", "db.internal", "5432")
	if specs != "8080:db.internal:5432,9000:db.internal:5432,9001:db.internal:5432" {
		t.Fatalf("unexpected specs: %s", specs)
	}
}

func TestBuildPortForwardSpecsFallsBackToSamePortWhenRemotePortIsEmpty(t *testing.T) {
	specs := buildPortForwardSpecs("8080-8081", "db.internal", "")
	if specs != "8080:db.internal:8080,8081:db.internal:8081" {
		t.Fatalf("unexpected specs: %s", specs)
	}
}

func TestUpdateSettingsKeePassPasswordPersistsOnBlankUpdate(t *testing.T) {
	store := memory.NewStore()
	service := NewService(store, nil, nil, nil)

	initial := store.Settings()
	initial.VaultProvider = "keepass"
	initial.KeePassDatabasePath = "/tmp/dev.kdbx"
	if err := store.StoreSecret(securestorage.KeePassPasswordKey(), "keepass-secret"); err != nil {
		t.Fatalf("store keepass password: %v", err)
	}
	if err := store.UpdateSettings(initial); err != nil {
		t.Fatalf("seed settings: %v", err)
	}

	updated := store.Settings()
	updated.KeePassPassword = ""
	if err := service.UpdateSettings(updated); err != nil {
		t.Fatalf("update settings: %v", err)
	}

	reloaded := store.Settings()
	if !reloaded.HasKeePassPassword {
		t.Fatal("expected keepass password flag to persist")
	}
	password, err := store.LoadSecret(securestorage.KeePassPasswordKey())
	if err != nil {
		t.Fatalf("load keepass password: %v", err)
	}
	if password != "keepass-secret" {
		t.Fatalf("expected keepass password to persist, got %q", password)
	}
}

func TestApplySSHForwardingSettingsUsesRemoteHostAndPort(t *testing.T) {
	store := memory.NewStore()
	target := sessions.Profile{
		ID:         "jump-host",
		Name:       "jump-host",
		ProtocolID: "ssh",
		Host:       "jump.internal",
		Port:       22,
		Username:   "ops",
	}
	source := sessions.Profile{
		ID:         "source-host",
		Name:       "source-host",
		ProtocolID: "ssh",
		Host:       "source.internal",
		Port:       22,
		Username:   "ops",
	}
	if err := store.UpsertSessionProfile(target); err != nil {
		t.Fatalf("seed target profile: %v", err)
	}
	if err := store.UpsertSessionProfile(source); err != nil {
		t.Fatalf("seed source profile: %v", err)
	}
	cfg := store.Settings()
	cfg.PortForwardRules = []settings.PortForwardRule{
		{LocalPort: "15432", RemoteHost: "db.internal", RemotePort: "5432", HostID: "jump-host", Enabled: true},
	}
	if err := store.UpdateSettings(cfg); err != nil {
		t.Fatalf("update settings: %v", err)
	}

	service := NewService(store, nil, nil, nil)
	withRules := service.applySSHForwardingSettings(source)
	if got := withRules.Options["local_forwards"]; got != "15432:db.internal:5432" {
		t.Fatalf("unexpected forward specs: %q", got)
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
