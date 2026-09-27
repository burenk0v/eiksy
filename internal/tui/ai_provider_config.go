package tui

import (
	"fmt"
	"strings"

	domainai "eiksy/internal/domain/ai"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type AIProviderConfigBackend interface {
	SaveCloudProvider(model, endpoint, token string) error
	SaveLocalProvider(downloadURL string) error
	DownloadLocalModel(downloadURL string) error
	StartLocalModel() error
	StopLocalModel() error
	ListCloudModels(endpoint, token string) ([]string, error)
}

type aiProviderConfigForm struct {
	providerIndex int
	class domainai.ProviderClass
	field int
	model string
	endpoint string
	token string
	downloadURL string
}

type aiProviderConfigDone struct{}
type aiProviderConfigError struct{ err error }
type aiProviderModelsDone struct{ models []string }
type aiProviderModelsError struct{ err error }
type aiProviderLocalActionDone struct{ action string }
type aiProviderLocalActionError struct{ err error }

func newAIProviderConfigForm(provider domainai.ProviderDescriptor, index int) *aiProviderConfigForm {
	return &aiProviderConfigForm{
		providerIndex: index,
		class: provider.Class,
		model: provider.Model,
		endpoint: provider.Endpoint,
		downloadURL: provider.DownloadURL,
	}
}

func (f *aiProviderConfigForm) isLocal() bool {
	return f.class == domainai.ProviderClassLocalOpenAI
}

func (f *aiProviderConfigForm) value() string {
	if f.isLocal() {
		return f.downloadURL
	}
	switch f.field {
	case 0:
		return f.model
	case 1:
		return f.endpoint
	case 2:
		return f.token
	}
	return ""
}

func (f *aiProviderConfigForm) setValue(value string) {
	if f.isLocal() {
		f.downloadURL = value
		return
	}
	switch f.field {
	case 0:
		f.model = value
	case 1:
		f.endpoint = value
	case 2:
		f.token = value
	}
}

func (m *Model) openAIProviderConfig() bool {
	providers := m.aiProviders()
	if len(providers) == 0 || m.aiProviderIndex < 0 || m.aiProviderIndex >= len(providers) {
		m.messages = append(m.messages, ChatMessage{Role: "System", Content: "No AI provider selected."})
		return false
	}
	m.aiProviderForm = newAIProviderConfigForm(providers[m.aiProviderIndex], m.aiProviderIndex)
	return true
}

func (m *Model) submitAIProviderConfig() tea.Cmd {
	form := m.aiProviderForm
	if form == nil {
		return nil
	}
	backend, ok := m.backend.(AIProviderConfigBackend)
	if !ok {
		m.messages = append(m.messages, ChatMessage{Role: "System", Content: "AI provider configuration unavailable."})
		m.aiProviderForm = nil
		return nil
	}
	if form.isLocal() {
		url := strings.TrimSpace(form.downloadURL)
		if url == "" {
			m.messages = append(m.messages, ChatMessage{Role: "System", Content: "Model URL is required."})
			return nil
		}
		m.aiProviderForm = nil
		return func() tea.Msg {
			if err := backend.SaveLocalProvider(url); err != nil {
				return aiProviderConfigError{err: err}
			}
			return aiProviderConfigDone{}
		}
	}
	model := strings.TrimSpace(form.model)
	endpoint := strings.TrimSpace(form.endpoint)
	if model == "" || endpoint == "" {
		m.messages = append(m.messages, ChatMessage{Role: "System", Content: "Cloud model and endpoint are required."})
		return nil
	}
	token := form.token
	m.aiProviderForm = nil
	return func() tea.Msg {
		if err := backend.SaveCloudProvider(model, endpoint, token); err != nil {
			return aiProviderConfigError{err: err}
		}
		return aiProviderConfigDone{}
	}
}

func (m *Model) loadAIProviderModels() tea.Cmd {
	backend, ok := m.backend.(AIProviderConfigBackend)
	if !ok {
		m.messages = append(m.messages, ChatMessage{Role: "System", Content: "Cloud model listing unavailable."})
		return nil
	}
	providers := m.aiProviders()
	if len(providers) == 0 || m.aiProviderIndex < 0 || m.aiProviderIndex >= len(providers) {
		return nil
	}
	provider := providers[m.aiProviderIndex]
	if provider.Class != domainai.ProviderClassOpenAICompatible {
		return nil
	}
	endpoint := strings.TrimSpace(provider.Endpoint)
	token := ""
	if m.aiProviderForm != nil {
		endpoint = strings.TrimSpace(m.aiProviderForm.endpoint)
		token = m.aiProviderForm.token
	}
	if endpoint == "" {
		m.messages = append(m.messages, ChatMessage{Role: "System", Content: "Cloud endpoint is required."})
		return nil
	}
	m.aiProviderStatus = "Loading cloud models..."
	return func() tea.Msg {
		models, err := backend.ListCloudModels(endpoint, token)
		if err != nil {
			return aiProviderModelsError{err: err}
		}
		return aiProviderModelsDone{models: models}
	}
}

func (m *Model) runAIProviderLocalAction() tea.Cmd {
	backend, ok := m.backend.(AIProviderConfigBackend)
	if !ok {
		m.messages = append(m.messages, ChatMessage{Role: "System", Content: "Local AI lifecycle unavailable."})
		return nil
	}
	providers := m.aiProviders()
	if len(providers) == 0 || m.aiProviderIndex < 0 || m.aiProviderIndex >= len(providers) {
		return nil
	}
	provider := providers[m.aiProviderIndex]
	if provider.Class != domainai.ProviderClassLocalOpenAI {
		return nil
	}
	action := "start"
	if provider.Status == "running" {
		action = "stop"
	}
	return func() tea.Msg {
		var err error
		if action == "start" {
			err = backend.StartLocalModel()
		} else {
			err = backend.StopLocalModel()
		}
		if err != nil {
			return aiProviderLocalActionError{err: err}
		}
		return aiProviderLocalActionDone{action: action}
	}
}

func (m *Model) downloadAIProviderModel() tea.Cmd {
	backend, ok := m.backend.(AIProviderConfigBackend)
	if !ok {
		m.messages = append(m.messages, ChatMessage{Role: "System", Content: "Local model download unavailable."})
		return nil
	}
	if m.aiProviderForm == nil || !m.aiProviderForm.isLocal() {
		return nil
	}
	url := strings.TrimSpace(m.aiProviderForm.downloadURL)
	if url == "" {
		m.messages = append(m.messages, ChatMessage{Role: "System", Content: "Model URL is required."})
		return nil
	}
	return func() tea.Msg {
		if err := backend.DownloadLocalModel(url); err != nil {
			return aiProviderLocalActionError{err: err}
		}
		return aiProviderLocalActionDone{action: "download"}
	}
}

func (m Model) renderAIProviderConfig(width, height int, title, key lipgloss.Style) string {
	form := m.aiProviderForm
	if form == nil {
		return ""
	}
	providers := m.aiProviders()
	providerName := "AI provider"
	if form.providerIndex >= 0 && form.providerIndex < len(providers) {
		providerName = providers[form.providerIndex].Name
	}
	var b strings.Builder
	fmt.Fprintf(&b, "%s\n\n", title.Render("CONFIGURE "+strings.ToUpper(providerName)))
	if form.isLocal() {
		value := form.downloadURL
		if form.field == 0 {
			value += "▌"
		}
		fmt.Fprintf(&b, "› Model URL: %s\n", value)
		b.WriteString("\nCtrl+S save   Ctrl+D download   Ctrl+R start/stop   Esc cancel")
	} else {
		values := []string{form.model, form.endpoint, strings.Repeat("*", len(form.token))}
		labels := []string{"Model", "Endpoint", "API token"}
		for i, label := range labels {
			marker := "  "
			if i == form.field {
				marker = "› "
				values[i] += "▌"
			}
			fmt.Fprintf(&b, "%s%-10s %s\n", marker, label, values[i])
		}
		b.WriteString("\n↑/↓ field   Enter save   Ctrl+L load models   Esc cancel")
		if len(m.aiCloudModels) > 0 {
			b.WriteString("\n\nAvailable models:\n")
			for _, model := range m.aiCloudModels {
				fmt.Fprintf(&b, "  %s\n", model)
			}
		}
	}
	if m.aiProviderStatus != "" {
		b.WriteString("\n\n")
		b.WriteString(m.aiProviderStatus)
	}
	return panelFixed(b.String(), width, height)
}
