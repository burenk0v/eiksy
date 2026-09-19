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

type SessionSelector interface {
	SelectChatSession(sessionID string) error
}

type SessionForker interface {
	ForkChatSession(sessionID, title string) (SessionRef, error)
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
	toolCalls []ToolCallView
	input     string
	palette   *CommandPalette
	approval  *agentai.ApprovalRequest

	sessions        []SessionRef
	activeSession   int
	sessionSelector SessionSelector
	sessionForker   SessionForker

	approvalResolver ApprovalResolver
}

type PaletteCommand struct {
	ID    string
	Title string
}

type CommandPalette struct {
	Query    string
	Selected int
}

var paletteCommands = []PaletteCommand{
	{ID: "next-tab", Title: "Next tab"},
	{ID: "previous-tab", Title: "Previous tab"},
	{ID: "next-session", Title: "Next session"},
	{ID: "previous-session", Title: "Previous session"},
	{ID: "fork-session", Title: "Fork active session"},
	{ID: "quit", Title: "Quit Eiksy"},
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

// WithChatSessions provides application-owned session metadata to the TUI.
// Message bodies are intentionally not part of this presentation model.
func (m Model) WithChatSessions(sessions []SessionRef) Model {
	m.sessions = append([]SessionRef(nil), sessions...)
	if m.activeSession >= len(m.sessions) {
		m.activeSession = 0
	}
	m.syncActiveSessionTab()
	return m
}

// WithSessionSelector connects session selection to the application service.
func (m Model) WithSessionSelector(selector SessionSelector) Model {
	m.sessionSelector = selector
	return m
}

// ActiveSession returns the currently highlighted chat session.
func (m Model) WithSessionForker(forker SessionForker) Model {
	m.sessionForker = forker
	return m
}

func (m Model) ActiveSession() *SessionRef {
	if m.activeSession < 0 || m.activeSession >= len(m.sessions) {
		return nil
	}
	session := m.sessions[m.activeSession]
	return &session
}

func (m Model) Init() tea.Cmd { return nil }

// WithApprovalResolver connects the TUI to the existing backend approval
// service without making the TUI responsible for policy or execution.
func (m Model) WithApprovalResolver(resolver ApprovalResolver) Model {
	m.approvalResolver = resolver
	return m
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case agentai.Event:
		m.handleAgentEvent(msg)
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
	case approvalResolutionDone:
		m.messages = append(m.messages, ChatMessage{Role: "System", Content: fmt.Sprintf("Approval %s applied.", msg.mode)})
	case approvalResolutionError:
		m.messages = append(m.messages, ChatMessage{Role: "System", Content: fmt.Sprintf("Approval failed: %v", msg.err)})
	case sessionSelectionDone:
		m.messages = append(m.messages, ChatMessage{Role: "System", Content: fmt.Sprintf("Session %s selected.", msg.sessionID)})
	case sessionSelectionError:
		m.messages = append(m.messages, ChatMessage{Role: "System", Content: fmt.Sprintf("Session selection failed: %v", msg.err)})
	case sessionForkDone:
		m.sessions = append(m.sessions, msg.session)
		m.activeSession = len(m.sessions) - 1
		m.syncActiveSessionTab()
		m.messages = nil
		m.toolCalls = nil
		m.messages = append(m.messages, ChatMessage{Role: "System", Content: fmt.Sprintf("Session %s forked.", msg.session.Title)})
	case sessionForkError:
		m.messages = append(m.messages, ChatMessage{Role: "System", Content: fmt.Sprintf("Session fork failed: %v", msg.err)})
	case tea.KeyMsg:
		if msg.Type == tea.KeyCtrlP {
			if m.palette == nil { m.palette = &CommandPalette{} } else { m.palette = nil }
			return m, nil
		}
		if m.palette != nil {
			return m.updateCommandPalette(msg)
		}
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
		case tea.KeyUp:
			m.selectPreviousSession()
		case tea.KeyDown:
			m.selectNextSession()
		case tea.KeyEnter:
			if m.activeTab == 0 && len(m.sessions) > 0 && strings.TrimSpace(m.input) == "" {
				if cmd := m.selectActiveSession(); cmd != nil {
					return m, cmd
				}
			} else {
				m.submitChatInput()
			}
		case tea.KeyBackspace:
			if len(m.input) > 0 {
				m.input = m.input[:len(m.input)-1]
			}
		case tea.KeyRunes:
			if len(msg.Runes) == 1 && (msg.Runes[0] == 'f' || msg.Runes[0] == 'F') && m.activeTab == 0 && strings.TrimSpace(m.input) == "" {
				if cmd := m.forkActiveSession(); cmd != nil { return m, cmd }
				break
			}
			if len(msg.Runes) == 1 && msg.Runes[0] >= 32 {
				m.input += string(msg.Runes)
			}
		}
	}
	return m, nil
}

func (m *Model) updateCommandPalette(msg tea.KeyMsg) (Model, tea.Cmd) {
	filtered := m.filteredPaletteCommands()
	switch msg.Type {
	case tea.KeyEsc:
		m.palette = nil
	case tea.KeyUp:
		if len(filtered) > 0 { m.palette.Selected = (m.palette.Selected - 1 + len(filtered)) % len(filtered) }
	case tea.KeyDown:
		if len(filtered) > 0 { m.palette.Selected = (m.palette.Selected + 1) % len(filtered) }
	case tea.KeyEnter:
		if len(filtered) > 0 {
			command := filtered[m.palette.Selected]
			m.palette = nil
			return *m, m.executePaletteCommand(command.ID)
		}
	case tea.KeyBackspace:
		if len(m.palette.Query) > 0 { m.palette.Query = m.palette.Query[:len(m.palette.Query)-1]; m.palette.Selected = 0 }
	case tea.KeyRunes:
		for _, r := range msg.Runes {
			if r >= 32 { m.palette.Query += string(r) }
		}
		m.palette.Selected = 0
	}
	return *m, nil
}

func (m Model) filteredPaletteCommands() []PaletteCommand {
	if m.palette == nil { return nil }
	query := strings.ToLower(strings.TrimSpace(m.palette.Query))
	if query == "" { return append([]PaletteCommand(nil), paletteCommands...) }
	result := make([]PaletteCommand, 0, len(paletteCommands))
	for _, command := range paletteCommands {
		if strings.Contains(strings.ToLower(command.Title), query) || strings.Contains(command.ID, query) { result = append(result, command) }
	}
	return result
}

func (m *Model) executePaletteCommand(id string) tea.Cmd {
	switch id {
	case "next-tab": m.selectNextTab()
	case "previous-tab": m.selectPreviousTab()
	case "next-session": m.selectNextSession()
	case "previous-session": m.selectPreviousSession()
	case "fork-session": return m.forkActiveSession()
	case "quit": return tea.Quit
	}
	return nil
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
	}
}

func (m *Model) resolveApproval(mode string) tea.Cmd {
	if m.approval == nil {
		return nil
	}
	requestID := m.approval.RequestID
	m.approval = nil
	if m.approvalResolver == nil {
		m.messages = append(m.messages, ChatMessage{Role: "System", Content: fmt.Sprintf("Approval unavailable for %s.", requestID)})
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

func (m *Model) selectPreviousSession() {
	if len(m.sessions) == 0 {
		return
	}
	m.activeSession = (m.activeSession - 1 + len(m.sessions)) % len(m.sessions)
	m.syncActiveSessionTab()
}

func (m *Model) selectNextSession() {
	if len(m.sessions) == 0 {
		return
	}
	m.activeSession = (m.activeSession + 1) % len(m.sessions)
	m.syncActiveSessionTab()
}

func (m *Model) syncActiveSessionTab() {
	if len(m.tabs) == 0 || len(m.sessions) == 0 {
		return
	}
	session := m.sessions[m.activeSession]
	m.tabs[0].BindSession(session.ID, session.Title)
}

func (m *Model) selectActiveSession() tea.Cmd {
	session := m.ActiveSession()
	if session == nil {
		return nil
	}
	if m.sessionSelector == nil {
		m.messages = append(m.messages, ChatMessage{Role: "System", Content: fmt.Sprintf("Session %s selected.", session.Title)})
		return nil
	}
	sessionID := session.ID
	selector := m.sessionSelector
	return func() tea.Msg {
		if err := selector.SelectChatSession(sessionID); err != nil {
			return sessionSelectionError{err: err}
		}
		return sessionSelectionDone{sessionID: sessionID}
	}
}

type sessionSelectionDone struct{ sessionID string }
type sessionSelectionError struct{ err error }

func (m *Model) forkActiveSession() tea.Cmd {
	session := m.ActiveSession()
	if session == nil { return nil }
	if m.sessionForker == nil {
		m.messages = append(m.messages, ChatMessage{Role: "System", Content: "Session forking unavailable."})
		return nil
	}
	sessionID := session.ID
	title := session.Title + " (fork)"
	forker := m.sessionForker
	return func() tea.Msg {
		created, err := forker.ForkChatSession(sessionID, title)
		if err != nil { return sessionForkError{err: err} }
		return sessionForkDone{session: created}
	}
}

type sessionForkDone struct{ session SessionRef }
type sessionForkError struct{ err error }

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
	if m.palette != nil {
		var palette strings.Builder
		palette.WriteString("  COMMAND PALETTE\n\n")
		fmt.Fprintf(&palette, "  > %s\n\n", m.palette.Query)
		filtered := m.filteredPaletteCommands()
		for i, command := range filtered {
			marker := "  "
			if i == m.palette.Selected { marker = "> " }
			fmt.Fprintf(&palette, "  %s%s\n", marker, command.Title)
		}
		content = palette.String() + "\n  Esc close   ↑/↓ select   Enter run"
	}
	if active == "Chat" {
		var chat strings.Builder
		if len(m.sessions) > 0 {
			chat.WriteString("  Sessions\n")
			for i, session := range m.sessions {
				marker := "  "
				if i == m.activeSession {
					marker = "> "
				}
				fmt.Fprintf(&chat, "  %s%s\n", marker, session.Title)
			}
			chat.WriteString("\n")
		}
		if m.approval != nil {
			chat.WriteString("  EIKSY ACTION\n\n")
			fmt.Fprintf(&chat, "  Tool: %s\n", m.approval.ToolID)
			fmt.Fprintf(&chat, "  Session: %s\n", m.approval.SessionID)
			fmt.Fprintf(&chat, "  Command: %s\n", m.approval.Command)
			if m.approval.Reason != "" {
				fmt.Fprintf(&chat, "  Reason: %s\n", m.approval.Reason)
			}
			chat.WriteString("  [Y] now  [S] session  [A] always  [N] deny\n\n")
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
	if m.palette != nil {
		var palette strings.Builder
		palette.WriteString("  COMMAND PALETTE\n\n")
		fmt.Fprintf(&palette, "  > %s\n\n", m.palette.Query)
		filtered := m.filteredPaletteCommands()
		for i, command := range filtered {
			marker := "  "
			if i == m.palette.Selected { marker = "> " }
			fmt.Fprintf(&palette, "  %s%s\n", marker, command.Title)
		}
		content = palette.String() + "\n  Esc close   ↑/↓ select   Enter run"
	}
	footer := "  enter send   ↑/↓ sessions   f fork   ←/→/tab tabs   ctrl+p commands   q quit"
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
