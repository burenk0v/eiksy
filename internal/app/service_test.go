package app

import (
	"bytes"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"eiksy/internal/securestorage"
	"eiksy/internal/storage/memory"
)

func TestStartCloudProviderAuthBuildsSourcegraphCallbackURL(t *testing.T) {
	service := NewService(memory.NewStore(), nil, nil); defer stopCloudAuthSessionForTest(service)
	session, err := service.StartCloudProviderAuth("https://sourcegraph.example.com/.api/llm/openai/v1")
	if err != nil { t.Fatalf("start cloud provider auth: %v", err) }
	if session.Status != "pending" { t.Fatalf("expected pending auth session, got %q", session.Status) }
	authURL, err := url.Parse(session.AuthURL); if err != nil { t.Fatalf("parse auth url: %v", err) }
	if authURL.Scheme != "https" || authURL.Host != "sourcegraph.example.com" { t.Fatalf("unexpected auth url origin: %s", session.AuthURL) }
	if authURL.Path != "/user/settings/tokens/new/callback" { t.Fatalf("unexpected auth url path: %s", authURL.Path) }
	if !strings.HasPrefix(authURL.Query().Get("requestFrom"), "CODY_CLI-") { t.Fatalf("unexpected requestFrom value: %q", authURL.Query().Get("requestFrom")) }
}

func TestStartCloudProviderAuthRejectsUnsupportedEndpoint(t *testing.T) {
	service := NewService(memory.NewStore(), nil, nil)
	_, err := service.StartCloudProviderAuth("https://api.openai.com/v1")
	if err == nil || !strings.Contains(err.Error(), "Sourcegraph/Cody-compatible") { t.Fatalf("expected unsupported endpoint error, got %v", err) }
}

func TestCloudProviderAuthSessionCompletesFromLocalhostCallback(t *testing.T) {
	store := memory.NewStore()
	service := NewService(store, nil, nil); defer stopCloudAuthSessionForTest(service)
	session, err := service.StartCloudProviderAuth("https://sourcegraph.example.com/.api/llm/openai/v1"); if err != nil { t.Fatalf("start cloud provider auth: %v", err) }
	authURL, err := url.Parse(session.AuthURL); if err != nil { t.Fatalf("parse auth url: %v", err) }
	requestFrom := authURL.Query().Get("requestFrom"); port := strings.TrimPrefix(requestFrom, "CODY_CLI-"); if port == requestFrom || port == "" { t.Fatalf("unexpected requestFrom value: %q", requestFrom) }
	resp, err := http.Get("http://localhost:" + port + "/api/sourcegraph/token?token=browser-token"); if err != nil { t.Fatalf("invoke callback: %v", err) }; resp.Body.Close()
	if resp.StatusCode != http.StatusOK { t.Fatalf("expected callback status 200, got %d", resp.StatusCode) }
	completed, err := service.GetCloudProviderAuthSession(session.ID); if err != nil { t.Fatalf("get auth session: %v", err) }
	if completed.Status != "completed" { t.Fatalf("expected completed auth session, got %q", completed.Status) }
	if completed.Token != "" { t.Fatalf("expected browser token to stay out of auth DTO, got %q", completed.Token) }
	token, err := store.LoadSecret(securestorage.AIProviderTokenKey("openai-compatible-cloud")); if err != nil { t.Fatalf("load callback token from secure storage: %v", err) }
	if token != "browser-token" { t.Fatalf("expected callback token in secure storage, got %q", token) }
}

func TestCloudProviderAuthSessionAcceptsPostedAccessToken(t *testing.T) {
	store := memory.NewStore()
	service := NewService(store, nil, nil); defer stopCloudAuthSessionForTest(service)
	session, err := service.StartCloudProviderAuth("https://sourcegraph.example.com/.api/llm/openai/v1"); if err != nil { t.Fatalf("start cloud provider auth: %v", err) }
	authURL, err := url.Parse(session.AuthURL); if err != nil { t.Fatalf("parse auth url: %v", err) }
	requestFrom := authURL.Query().Get("requestFrom"); port := strings.TrimPrefix(requestFrom, "CODY_CLI-"); if port == requestFrom || port == "" { t.Fatalf("unexpected requestFrom value: %q", requestFrom) }
	resp, err := http.Post("http://127.0.0.1:"+port+"/api/sourcegraph/token", "application/json", bytes.NewBufferString(`{"accessToken":"posted-token"}`)); if err != nil { t.Fatalf("post callback: %v", err) }; resp.Body.Close()
	if resp.StatusCode != http.StatusOK { t.Fatalf("expected callback status 200, got %d", resp.StatusCode) }
	completed, err := service.GetCloudProviderAuthSession(session.ID); if err != nil { t.Fatalf("get auth session: %v", err) }
	if completed.Status != "completed" { t.Fatalf("expected completed auth session, got %q", completed.Status) }
	if completed.Token != "" { t.Fatalf("expected posted token to stay out of auth DTO, got %q", completed.Token) }
	token, err := store.LoadSecret(securestorage.AIProviderTokenKey("openai-compatible-cloud")); if err != nil { t.Fatalf("load posted token from secure storage: %v", err) }
	if token != "posted-token" { t.Fatalf("expected posted token in secure storage, got %q", token) }
}

func TestClearChatKeepsMessageSliceUsable(t *testing.T) {
	service := NewService(memory.NewStore(), nil, nil)
	if err := service.SendChatMessage(nil, "hello", ""); err == nil || !strings.Contains(err.Error(), "no AI provider is configured") { t.Fatalf("expected provider configuration error, got %v", err) }
	service.ClearChat(); state := service.GetShellState()
	if state.AI.Messages == nil { t.Fatal("expected clear chat to leave an empty message slice") }; if len(state.AI.Messages) != 0 { t.Fatalf("expected chat messages to be cleared, got %d", len(state.AI.Messages)) }; if state.AI.ChatSessionID == "" { t.Fatal("expected chat session id to be regenerated") }
}

func TestResolveCommandPolicyRequestWithSessionApprovalExecutesCommand(t *testing.T) {
	store := memory.NewStore(); profile := sessions.Profile{ID: "ssh-host", Name: "ssh-host", ProtocolID: "ssh", Host: "host", Port: 22, Username: "ops"}; if err := store.UpsertSessionProfile(profile); err != nil { t.Fatalf("seed profile: %v", err) }
	ssh := &recordingSSHManager{}; service := NewService(store, ssh, nil); tab, err := service.LaunchSession(profile.ID); if err != nil { t.Fatalf("launch session: %v", err) }