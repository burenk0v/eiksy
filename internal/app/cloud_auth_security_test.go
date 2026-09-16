package app

import (
	"net/http"
	"net/url"
	"strings"
	"testing"

	"eiksy/internal/securestorage"
	"eiksy/internal/storage/memory"
)

func TestCloudProviderAuthStoresTokenOutsideWailsState(t *testing.T) {
	service := NewService(memory.NewStore(), nil, nil)
	defer stopCloudAuthSessionForTest(service)

	session, err := service.StartCloudProviderAuth("https://sourcegraph.example.com/.api/llm/openai/v1")
	if err != nil {
		t.Fatalf("start cloud provider auth: %v", err)
	}
	authURL, err := url.Parse(session.AuthURL)
	if err != nil {
		t.Fatalf("parse auth url: %v", err)
	}
	requestFrom := authURL.Query().Get("requestFrom")
	port := strings.TrimPrefix(requestFrom, "CODY_CLI-")
	if port == requestFrom || port == "" {
		t.Fatalf("unexpected requestFrom value: %q", requestFrom)
	}

	resp, err := http.Get("http://localhost:" + port + "/api/sourcegraph/token?token=browser-secret-token")
	if err != nil {
		t.Fatalf("invoke callback: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected callback status 200, got %d", resp.StatusCode)
	}

	completed, err := service.GetCloudProviderAuthSession(session.ID)
	if err != nil {
		t.Fatalf("get completed auth session: %v", err)
	}
	if completed.Status != "completed" {
		t.Fatalf("expected completed auth session, got %q", completed.Status)
	}
	if completed.Token != "" {
		t.Fatal("browser authorization token must not cross the Wails-facing auth session DTO")
	}

	token, err := service.store.LoadSecret(securestorage.AIProviderTokenKey("openai-compatible-cloud"))
	if err != nil {
		t.Fatalf("load browser authorization token from secure storage: %v", err)
	}
	if token != "browser-secret-token" {
		t.Fatalf("expected token to be stored in secure storage, got %q", token)
	}
}
