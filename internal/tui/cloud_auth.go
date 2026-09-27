package tui

import (
	"fmt"
	"net/url"
	"os/exec"
	"runtime"
	"strings"

	appservice "eiksy/internal/app"

	tea "github.com/charmbracelet/bubbletea"
)

type CloudProviderAuthBackend interface {
	StartCloudProviderAuth(endpoint string) (appservice.CloudProviderAuthSession, error)
	GetCloudProviderAuthSession(sessionID string) (appservice.CloudProviderAuthSession, error)
}

type aiCloudAuthStarted struct{ session appservice.CloudProviderAuthSession }
type aiCloudAuthPoll struct{ sessionID string }
type aiCloudAuthPending struct{ sessionID string }
type aiCloudAuthDone struct{ message string }
type aiCloudAuthError struct{ err error }

func supportsCloudProviderBrowserAuth(endpoint string) bool {
	normalized := strings.TrimSpace(endpoint)
	if normalized == "" { return false }
	u, err := url.Parse(normalized)
	if err != nil { return false }
	return strings.Contains(u.Path, "/.api/llm/openai/") || strings.HasSuffix(u.Path, "/.api/llm/openai") || strings.Contains(u.Path, "/api/llm/openai/") || strings.HasSuffix(u.Path, "/api/llm/openai")
}

func openBrowserURL(rawURL string) error {
	if strings.TrimSpace(rawURL) == "" { return fmt.Errorf("browser authorization URL is empty") }
	switch runtime.GOOS {
	case "windows":
		return exec.Command("rundll32", "url.dll,FileProtocolHandler", rawURL).Start()
	case "darwin":
		return exec.Command("open", rawURL).Start()
	default:
		return exec.Command("xdg-open", rawURL).Start()
	}
}

func (m *Model) startCloudProviderAuth() tea.Cmd {
	backend, ok := m.backend.(CloudProviderAuthBackend)
	if !ok { m.aiCloudAuthMessage = "Browser authorization unavailable."; return nil }
	if m.aiProviderForm == nil || m.aiProviderForm.isLocal() { return nil }
	endpoint := strings.TrimSpace(m.aiProviderForm.endpoint)
	if endpoint == "" { m.aiCloudAuthMessage = "Cloud endpoint is required."; return nil }
	if !supportsCloudProviderBrowserAuth(endpoint) { m.aiCloudAuthMessage = "Browser authorization is not supported for this endpoint."; return nil }
	m.aiCloudAuthPending = true
	m.aiCloudAuthMessage = "Starting browser authorization..."
	return func() tea.Msg {
		session, err := backend.StartCloudProviderAuth(endpoint)
		if err != nil { return aiCloudAuthError{err: err} }
		return aiCloudAuthStarted{session: session}
	}
}

func (m *Model) pollCloudProviderAuth(sessionID string) tea.Cmd {
	backend, ok := m.backend.(CloudProviderAuthBackend)
	if !ok { return nil }
	return func() tea.Msg {
		session, err := backend.GetCloudProviderAuthSession(sessionID)
		if err != nil { return aiCloudAuthError{err: err} }
		switch session.Status {
		case "completed":
			return aiCloudAuthDone{message: nonEmpty(session.Message, "Browser authorization completed.")}
		case "failed", "expired":
			return aiCloudAuthError{err: fmt.Errorf("%s", nonEmpty(session.Message, "browser authorization did not complete"))}
		default:
			return aiCloudAuthPoll{sessionID: sessionID}
		}
	}
}

