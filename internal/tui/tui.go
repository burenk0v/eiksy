package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

type Tab struct {
	Title string
}

type Model struct {
	width     int
	height    int
	tabs      []Tab
	activeTab int
}

func NewModel() Model {
	return Model{
		tabs: []Tab{
			{Title: "Chat"},
			{Title: "Terminal"},
			{Title: "Files"},
			{Title: "Tools"},
		},
	}
}

func (m Model) Init() tea.Cmd { return nil }

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		case "left", "h":
			m.selectPreviousTab()
		case "right", "l", "tab":
			m.selectNextTab()
		}
	}
	return m, nil
}

func (m *Model) selectNextTab() {
	if len(m.tabs) == 0 {
		return
	}
	m.activeTab = (m.activeTab + 1) % len(m.tabs)
}

func (m *Model) selectPreviousTab() {
	if len(m.tabs) == 0 {
		return
	}
	m.activeTab = (m.activeTab - 1 + len(m.tabs)) % len(m.tabs)
}

func (m Model) View() string {
	width, height := m.width, m.height
	if width < 1 {
		width = 80
	}
	if height < 1 {
		height = 24
	}

	var tabBar strings.Builder
	for i, tab := range m.tabs {
		if i > 0 {
			tabBar.WriteString("  ")
		}
		if i == m.activeTab {
			fmt.Fprintf(&tabBar, "[%s]", tab.Title)
		} else {
			tabBar.WriteString(tab.Title)
		}
	}

	active := "Chat"
	if len(m.tabs) > 0 && m.activeTab >= 0 && m.activeTab < len(m.tabs) {
		active = m.tabs[m.activeTab].Title
	}

	header := " EIKSY  Think. Connect. Operate."
	content := fmt.Sprintf("  %s view\n\n  Tabs are presentation state only. Sessions and application services remain outside the TUI.", active)
	footer := "  ←/h previous   →/l/tab next   q quit"

	return strings.Join([]string{
		header,
		"",
		"  " + tabBar.String(),
		"",
		content,
		fmt.Sprintf("\n  %dx%d", width, height),
		footer,
	}, "\n")
}

func Run() error {
	_, err := tea.NewProgram(NewModel(), tea.WithAltScreen()).Run()
	return err
}
