package tui

import (
	"fmt"
	"strings"

	agentai "eiksy/internal/ai"
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

type ToolCallView struct {
	Name   string
	Output string
	Status string
}

type ApprovalResolver interface {
	ResolveApproval(requestID, mode string) error
}

type Model struct {
	width     int
	height    int
	tabs      []Tab
	activeTab int
	messages  []ChatMessage
	toolCalls       []ToolCallView
	input           string
	approval        *agentai.ApprovalRequest
	approvalResolver ApprovalResolver
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

// WithApprovalResolver connects the TUI to the existing backend approval
// service without making the TUI responsible for policy or execution.
func (m Model) WithApprovalResolver(resolver ApprovalResolver) Model {
	m.approvalResolver = resolver
	return m
}

func (m Model) Init() tea.Cmd { return nil }

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case agentai.Event:
		m.handleAgentEvent(msg)
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
	case tea.KeyMsg:
		if msg.Type == tea.KeyCtrlC || (msg.Type == tea.KeyRunes && len(msg.Runes) == 1 && msg.Runes[0] == 'q' && m.approval == nil) {
			return m, tea.Quit
		}
		if m.approval != nil && msg.Type == tea.KeyRunes && len(msg.Runes) == 1 {
			switch msg.Runes[0] {
			case 'y', 'Y':
				return m, m.resolveApproval("now")
			case 's', 'S':
				return m, m.resolveApproval("session")
			case 'a', 'A':
				return m, m.resolveApproval("always")
			case 'n', 'N':
				return m, m.resolveApproval("deny")
			}
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

func (m *Model) handleAgentEvent(event agentai.Event) {
	switch event.Type {
	case agentai.EventTextDelta:
		if event.Content != "" {
			m.messages = append(m.messages, ChatMessage{Role: "AI", Content: event.Content})
		}
	case agentai.EventToolStarted:
		name := strings.TrimSpace(event.Tool)
		if name == "" {
			name = "unknown"
		}
		m.toolCalls = append(m.toolCalls, ToolCallView{Name: name, Status: "running"})
	case agentai.EventToolOutput:
		if len(m.toolCalls) > 0 {
			m.toolCalls[len(m.toolCalls)-1].Output = event.Content
		}
	case agentai.EventToolFinished:
		if len(m.toolCalls) > 0 {
			call := &m.toolCalls[len(m.toolCalls)-1]
			call.Status = "finished"
			if strings.TrimSpace(event.Tool) != "" {
				call.Name = strings.TrimSpace(event.Tool)
			}
		}
	case agentai.EventApprovalRequired:
		if event.Approval != nil {
			request := *event.Approval
			m.approval = &request
		}
	case agentai.EventError:
		content := "AI error"
		if event.Err != nil {
			content = event.Err.Error()
		}
		m.messages = append(m.messages, ChatMessage{Role: "System", Content: content})
	case agentai.EventCancellation:
		m.messages = append(m.messages, ChatMessage{Role: "System", Content: "AI request cancelled"})
	case approvalResolutionDone:
		m.messages = append(m.messages, ChatMessage{Role: "System", Content: fmt.Sprintf("Approval %s applied.", msg.mode)})
	case approvalResolutionError:
		m.messages = append(m.messages, ChatMessage{Role: "System", Content: fmt.Sprintf("Approval failed: %v", msg.err)})
	}
}

func (m *Model) resolveApproval(mode string) tea.Cmd {
	if m.approval == nil {
		return nil
	}
	requestID := m.approval.RequestID
	m.approval = nil
	if m.approvalResolver == nil {
		m.messages = append(m.messages, ChatMessage{Role: "System", Content: fmt.Sprintf("Approval %s queued for %s.", mode, requestID)})
		return nil
	}
	resolver := m.approvalResolver
	return func() tea.Msg {
		if err := resolver.ResolveApproval(requestID, mode); err != nil {
			return approvalResolutionError{err: err}
		}
		return approvalResolutionDone{mode: mode}
	}
}

type approvalResolutionDone struct{ mode string }
type approvalResolutionError struct{ err error }

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
		if m.approval != nil {
			chat.WriteString("  EIKSY ACTION\\n\\n")
			fmt.Fprintf(&chat, "  Tool: %s\\n", m.approval.ToolID)
			fmt.Fprintf(&chat, "  Session: %s\\n", m.approval.SessionID)
			fmt.Fprintf(&chat, "  Command: %s\\n", m.approval.Command)
			if m.approval.Reason != "" {
				fmt.Fprintf(&chat, "  Reason: %s\\n", m.approval.Reason)
			}
			chat.WriteString("  \\[Y\\] now  \\[S\\] session  \\[A\\] always  \\[N\\] deny\\n\\n")
		}

		if len(m.messages) == 0 && len(m.toolCalls) == 0 {
			chat.WriteString("  No messages yet. Ask Eiksy something.")
		} else {
			for _, message := range m.messages {
				fmt.Fprintf(&chat, "  %s: %s\n", message.Role, message.Content)
			}
		}
		for _, tool := range m.toolCalls {
			fmt.Fprintf(&chat, "  [tool:%s] %s\n", tool.Name, tool.Status)
			if tool.Output != "" {
				fmt.Fprintf(&chat, "    output: %s\n", tool.Output)
			}
		}
		content = "  Chat\n\n" + chat.String() + fmt.Sprintf("\n  > %s", m.input)
	}
	footer := "  enter send   ←/h previous   →/l/tab next   q quit"
	if m.approval != nil {
		footer = "  approval: y now   s session   a always   n deny"
	}

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
