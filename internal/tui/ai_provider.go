package tui

import (
	"fmt"
	"strings"

	domainai "eiksy/internal/domain/ai"
	tea "github.com/charmbracelet/bubbletea"
)

type AIProviderBackend interface {
	SelectAIProvider(providerID string) error
}

type aiProviderSelectDone struct {
	providerID string
}

type aiProviderSelectError struct {
	err error
}

func (m Model) aiProviders() []domainai.ProviderDescriptor {
	if m.aiBackend == nil {
		return nil
	}
	state := m.aiBackend.GetShellState()
	return state.AI.Providers
}

func (m Model) aiProviderCount() int {
	return len(m.aiProviders())
}

func (m Model) selectAIProvider() tea.Cmd {
	backend, ok := m.backend.(AIProviderBackend)
	if !ok {
		return nil
	}
	providers := m.aiProviders()
	if len(providers) == 0 {
		return nil
	}
	index := m.aiProviderIndex
	if index < 0 || index >= len(providers) {
		index = 0
	}
	provider := providers[index]
	return func() tea.Msg {
		if err := backend.SelectAIProvider(provider.ID); err != nil {
			return aiProviderSelectError{err: err}
		}
		return aiProviderSelectDone{providerID: provider.ID}
	}
}

func (m Model) aiProviderView() string {
	providers := m.aiProviders()
	if len(providers) == 0 {
		return "No AI providers available."
	}
	var b strings.Builder
	for i, provider := range providers {
		marker := "  "
		if i == m.aiProviderIndex {
			marker = "› "
		}
		selected := ""
		if provider.Selected {
			selected = " *"
		}
		fmt.Fprintf(&b, "%s%s%s  [%s]  %s\n", marker, provider.Name, selected, provider.Status, provider.Model)
	}
	return strings.TrimSuffix(b.String(), "\n")
}
