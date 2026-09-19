package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

type SessionRef struct {
	ID    string
	Title string
}

type Tab struct {
	Title   string
	Session *SessionRef
}

func (t *Tab) BindSession(sessionID, title string) {
	sessionID = strings.TrimSpace(sessionID)
	if sessionID == "" {
		t.Session = nil
		return
	}
	t.Session = &SessionRef{ID: sessionID, Title: strings.TrimSpace(title)}
}

func (t *Tab) UnbindSession() {
	t.Session = nil
}

type ChatMessage struct {
	Role    string
	Content string
}

type Model struct {
	width     int
	height    int
	tabs      []Tab
	activeTab int
	messages  []ChatMessage
	input     string
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
		if msg.Type == tea.KeyCtrlC || (msg.Type == tea.KeyRunes && len(msg.Runes) == 1 && msg.Runes[0] == 'q') {
			return m, tea.Quit
		}
		switch msg.Type {
		case tea.KeyLeft:
			m.selectPreviousTab()
		case tea.KeyRight, tea.KeyTab:
			m.selectNextTab()
		case tea.KeyBackspace:
			if len(m.input) > 0 {
				m.input = m.input[:len(m.input)-1]
			}
		case tea.KeyEnter:
			m.submitChatInput()
		case tea.KeyRunes:
			if len(msg.Runes) == 1 && msg.Runes[0] >= 32 {
				m.input += string(msg.Runes)
			}
		}
	}
	return m, nil
}

func (m *Model) submitChatInput() {
	content := strings.TrimSpace(m.input)
	if content == "" || m.activeTab != 0 {
		return
	}
	m.messages = append(m.messages, ChatMessage{Role: "You", Content: content})
	m.input = ""
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
	if active == "Chat" {
		var chat strings.Builder
		if len(m.messages) == 0 {
			chat.WriteString("  No messages yet. Ask Eiksy something.")
		} else {
			for _, message := range m.messages {
				fmt.Fprintf(&chat, "  %s: %s\\n", message.Role, message.Content)
			}
		}
		content = "  Chat\\n\\n" + chat.String() + fmt.Sprintf("\\n  > %s", m.input)
	}
	footer := "  enter send   ←/h previous   →/l/tab next   q quit"

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
