package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

type Model struct {
	width  int
	height int
}

func NewModel() Model { return Model{} }

func (m Model) Init() tea.Cmd { return nil }

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		}
	}
	return m, nil
}

func (m Model) View() string {
	width, height := m.width, m.height
	if width < 1 { width = 80 }
	if height < 1 { height = 24 }

	header := " EIKSY  Think. Connect. Operate."
	content := "  TUI foundation\n\n  Tabs and application views will be added in subsequent PRs."
	footer := "  q quit"

	bodyHeight := height - 3
	if bodyHeight < 1 { bodyHeight = 1 }

	return strings.Join([]string{
		header,
		content,
		fmt.Sprintf("\n  %dx%d", width, height),
		footer,
	}, "\n")
}

func Run() error {
	_, err := tea.NewProgram(NewModel(), tea.WithAltScreen()).Run()
	return err
}
