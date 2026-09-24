package tui

import (
	"fmt"
	"strings"

	agentai "eiksy/internal/ai"
	domainai "eiksy/internal/domain/ai"
	domainsettings "eiksy/internal/domain/settings"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
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

type FileEntry struct {
	Name string
	Kind string
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

type Backend interface {
	ListChatSessions() []domainai.ChatSession
	CreateChatSession(title string) (domainai.ChatSession, error)
	SelectChatSession(sessionID string) error
	ForkChatSession(sessionID, title string) (domainai.ChatSession, error)
	GetSettings() domainsettings.AppSettings
	UpdateSettings(domainsettings.AppSettings) error
}

type Model struct {
	width     int
	height    int
	tabs      []Tab
	activeTab int
	messages  []ChatMessage
	toolCalls []ToolCallView
	input     string
	terminalLines []string
	terminalInput string
	fileEntries []FileEntry
	filePath    string
	activeView  string
	palette   *CommandPalette
	shortcuts bool
	approval  *agentai.ApprovalRequest

	sessions        []SessionRef
	activeSession   int
	sessionSelector SessionSelector
	sessionForker   SessionForker

	approvalResolver ApprovalResolver
	backend         Backend
	settings        domainsettings.AppSettings
	settingsIndex   int
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
			{Title: "Sessions"},
			{Title: "Terminal"},
			{Title: "Files"},
			{Title: "Tools"},
			{Title: "Settings"},
			{Title: "AI"},
		},
	}
}

// WithChatSessions provides application-owned session metadata to the TUI.
// Message bodies are intentionally not part of this presentation model.
func (m Model) WithBackend(backend Backend) Model {
	m.backend = backend
	if backend != nil {
		m.settings = backend.GetSettings()
		m.sessionSelector = backend
		m.sessionForker = backend
		m.refreshSessions()
	}
	return m
}

func (m Model) WithChatSessions(sessions []SessionRef) Model {
	m.sessions = append([]SessionRef(nil), sessions...)
	if m.activeSession >= len(m.sessions) {
		m.activeSession = 0
	}
	m.syncActiveSessionTab()
	return m
}

// WithTerminalSession binds a connected runtime session to the Terminal view.
func (m Model) WithTerminalSession(sessionID, title string) Model {
	if len(m.tabs) > 1 {
		m.tabs[1].BindSession(sessionID, title)
	}
	return m
}

func (m Model) hasTerminalSession() bool {
	return len(m.tabs) > 1 && m.tabs[1].Session != nil
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
	case sessionCreateDone:
		m.sessions = append(m.sessions, msg.session)
		m.activeSession = len(m.sessions) - 1
		m.syncActiveSessionTab()
		m.messages = append(m.messages, ChatMessage{Role: "System", Content: fmt.Sprintf("Session %s created.", msg.session.Title)})
	case settingsUpdateDone:
		m.settings = msg.settings
	case settingsUpdateError:
		m.messages = append(m.messages, ChatMessage{Role: "System", Content: fmt.Sprintf("Settings update failed: %v", msg.err)})
	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyF2:
			m.activeTab = 0
			m.activeView = ""
			return m, nil
		case tea.KeyF3:
			m.activeTab = 1
			m.activeView = ""
			return m, nil
		case tea.KeyF4:
			m.activeTab = 2
			m.activeView = ""
			return m, nil
		case tea.KeyF5:
			m.activeTab = 3
			m.activeView = "tools"
			return m, nil
		case tea.KeyF6:
			m.activeTab = 4
			m.activeView = ""
			return m, nil
		case tea.KeyF7:
			m.activeTab = 5
			m.activeView = ""
			return m, nil
		}
		if msg.Type == tea.KeyCtrlP {
			m.shortcuts = false
			if m.palette == nil { m.palette = &CommandPalette{} } else { m.palette = nil }
			return m, nil
		}
		if m.shortcuts {
			if msg.Type == tea.KeyEsc || (msg.Type == tea.KeyRunes && len(msg.Runes) == 1 && msg.Runes[0] == '?') {
				m.shortcuts = false
			}
			return m, nil
		}
		if m.palette != nil {
			return m.updateCommandPalette(msg)
		}
		if msg.Type == tea.KeyCtrlC || msg.Type == tea.KeyCtrlQ || msg.Type == tea.KeyF10 {
			return m, tea.Quit
		}
		if msg.Type == tea.KeyRunes && len(msg.Runes) == 1 && msg.Runes[0] == '?' && m.approval == nil && strings.TrimSpace(m.input) == "" {
			m.shortcuts = true
			return m, nil
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
		case tea.KeyCtrlN:
			if m.activeTab == 0 && strings.TrimSpace(m.input) == "" {
				if cmd := m.createChatSession(); cmd != nil { return m, cmd }
			}
		case tea.KeyLeft:
			m.activeView = ""
			m.selectPreviousTab()
		case tea.KeyRight, tea.KeyTab:
			m.activeView = ""
			m.selectNextTab()
		case tea.KeyShiftTab:
			m.activeView = ""
			m.selectPreviousTab()
		case tea.KeyUp:
			if m.activeTab == 4 { m.settingsIndex = (m.settingsIndex - 1 + m.settingsCount()) % m.settingsCount() } else { m.selectPreviousSession() }
		case tea.KeyDown:
			if m.activeTab == 4 { m.settingsIndex = (m.settingsIndex + 1) % m.settingsCount() } else { m.selectNextSession() }
		case tea.KeyEnter:
			if m.activeTab == 4 {
				if cmd := m.toggleSetting(); cmd != nil { return m, cmd }
			} else if m.activeTab == 1 {
				m.submitTerminalInput()
			} else if m.activeTab == 0 && len(m.sessions) > 0 && strings.TrimSpace(m.input) == "" {
				if cmd := m.selectActiveSession(); cmd != nil {
					return m, cmd
				}
			} else {
				m.submitChatInput()
			}
		case tea.KeyBackspace:
			if m.activeTab == 1 {
				if len(m.terminalInput) > 0 {
					m.terminalInput = m.terminalInput[:len(m.terminalInput)-1]
				}
			} else if len(m.input) > 0 {
				m.input = m.input[:len(m.input)-1]
			}
		case tea.KeyCtrlF:
			if m.activeTab == 0 && strings.TrimSpace(m.input) == "" {
				if cmd := m.forkActiveSession(); cmd != nil { return m, cmd }
			}
		case tea.KeyRunes:
			if m.activeTab == 1 {
				if !m.hasTerminalSession() {
					break
				}
				for _, r := range msg.Runes {
					if r >= 32 { m.terminalInput += string(r) }
				}
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

func (m Model) WithFileEntries(path string, entries []FileEntry) Model {
	m.filePath = strings.TrimSpace(path)
	m.fileEntries = append([]FileEntry(nil), entries...)
	return m
}

func (m *Model) submitTerminalInput() {
	if !m.hasTerminalSession() {
		return
	}
	content := strings.TrimSpace(m.terminalInput)
	if content == "" { return }
	m.terminalLines = append(m.terminalLines, "$ "+content)
	m.terminalInput = ""
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
	if width < 80 { width = 80 }
	if height < 24 { height = 24 }

	const headerHeight, tabsHeight, statusHeight, footerHeight = 2, 3, 1, 2
	bodyHeight := height - headerHeight - tabsHeight - statusHeight - footerHeight
	if bodyHeight < 10 { bodyHeight = 10 }
	const sidebarWidth = 27
	mainWidth := width - sidebarWidth - 3
	if mainWidth < 30 { mainWidth = 30 }

	border := lipgloss.Color("240")
	accent := lipgloss.Color("81")
	muted := lipgloss.Color("245")
	activeBg := lipgloss.Color("24")
	panelTitle := lipgloss.NewStyle().Bold(true).Foreground(accent)
	key := lipgloss.NewStyle().Foreground(accent).Bold(true)

	header := lipgloss.NewStyle().Width(width).Height(headerHeight).
		Background(lipgloss.Color("235")).Foreground(lipgloss.Color("255")).Padding(0, 2).
		Render("EIKSY  Think. Connect. Operate.")

	tabs := make([]string, 0, len(m.tabs))
	for i, tab := range m.tabs {
		label := fmt.Sprintf("F%d %s", i+2, tab.Title)
		style := lipgloss.NewStyle().Foreground(muted).Padding(0, 2)
		if i == m.activeTab {
			style = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("255")).Background(activeBg).Padding(0, 2)
		}
		tabs = append(tabs, style.Render(label))
	}
	tabRow := lipgloss.NewStyle().Width(width).Height(tabsHeight).BorderBottom(true).BorderForeground(border).
		Render(lipgloss.JoinHorizontal(lipgloss.Bottom, tabs...))

	body := lipgloss.JoinHorizontal(lipgloss.Top,
		m.renderSidebar(sidebarWidth-2, bodyHeight-2, panelTitle, key),
		" ",
		m.renderMainPanel(mainWidth, bodyHeight-2, panelTitle, key),
	)

	if m.shortcuts { body = m.renderHelp(width-2, bodyHeight-2, panelTitle) }
	if m.palette != nil { body = m.renderPalette(width-2, bodyHeight-2, panelTitle, key) }

	active := m.currentViewName()
	statusText := fmt.Sprintf(" %s  |  Session %d/%d", active, m.activeSession+1, len(m.sessions))
	if len(m.sessions) == 0 { statusText = fmt.Sprintf(" %s  |  No session selected", active) }
	statusBar := lipgloss.NewStyle().Foreground(muted).Width(width).Height(statusHeight).Render(statusText)

	footerText := key.Render("F2-F7") + " switch view   " + key.Render("Enter") + " select/send   " +
		key.Render("Ctrl+P") + " menu   " + key.Render("F10") + " exit   " + key.Render("?") + " help"
	footer := lipgloss.NewStyle().Width(width).Height(footerHeight).BorderTop(true).BorderForeground(border).
		Padding(0, 1).Render(footerText)

	return lipgloss.JoinVertical(lipgloss.Left, header, tabRow, body, statusBar, footer)
}

func (m Model) settingsCount() int { return 3 }

func boolLabel(value bool) string { if value { return "ON" }; return "OFF" }
func nonEmpty(value, fallback string) string { if strings.TrimSpace(value) == "" { return fallback }; return value }

func (m *Model) refreshSessions() {
	if m.backend == nil { return }
	items := m.backend.ListChatSessions()
	m.sessions = make([]SessionRef, 0, len(items))
	for _, item := range items { m.sessions = append(m.sessions, SessionRef{ID: item.ID, Title: item.Title}) }
	if m.activeSession >= len(m.sessions) { m.activeSession = 0 }
	m.syncActiveSessionTab()
}

func (m *Model) createChatSession() tea.Cmd {
	if m.backend == nil { m.messages = append(m.messages, ChatMessage{Role: "System", Content: "Session creation unavailable."}); return nil }
	backend := m.backend
	return func() tea.Msg {
		created, err := backend.CreateChatSession("New session")
		if err != nil { return sessionForkError{err: err} }
		return sessionCreateDone{session: SessionRef{ID: created.ID, Title: created.Title}}
	}
}

type sessionCreateDone struct{ session SessionRef }

func (m *Model) toggleSetting() tea.Cmd {
	if m.backend == nil { return nil }
	updated := m.settings
	switch m.settingsIndex {
	case 0: updated.PromptBeforeAI = !updated.PromptBeforeAI
	case 1: updated.AllowCloudModels = !updated.AllowCloudModels
	case 2:
		if strings.EqualFold(updated.Theme, "dark") { updated.Theme = "light" } else { updated.Theme = "dark" }
	}
	backend := m.backend
	return func() tea.Msg {
		if err := backend.UpdateSettings(updated); err != nil { return settingsUpdateError{err: err} }
		return settingsUpdateDone{settings: updated}
	}
}

type settingsUpdateDone struct{ settings domainsettings.AppSettings }
type settingsUpdateError struct{ err error }

func (m Model) currentViewName() string {
	if m.activeTab < 0 || m.activeTab >= len(m.tabs) { return "Sessions" }
	return m.tabs[m.activeTab].Title
}

func panelFixed(content string, width, height int) string {
	return lipgloss.NewStyle().Border(lipgloss.NormalBorder()).BorderForeground(lipgloss.Color("240")).
		Padding(0, 1).Width(width).Height(height).Render(content)
}

func (m Model) renderSidebar(width, height int, title, key lipgloss.Style) string {
	var b strings.Builder
	b.WriteString(title.Render("SESSIONS")); b.WriteString("\n\n")
	if len(m.sessions) == 0 {
		b.WriteString("No sessions\n")
	} else {
		for i, session := range m.sessions {
			marker := "  "; if i == m.activeSession { marker = "› " }
			name := session.Title; if name == "" { name = session.ID }
			line := marker + name
			if i == m.activeSession {
				line = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("255")).Background(lipgloss.Color("24")).Width(width).Render(line)
			}
			b.WriteString(line); b.WriteByte('\n')
		}
	}
	b.WriteString("\n"); b.WriteString(title.Render("NAVIGATION")); b.WriteString("\n\n")
	b.WriteString(key.Render("↑/↓")); b.WriteString(" sessions\n")
	b.WriteString(key.Render("←/→")); b.WriteString(" tabs\n")
	b.WriteString(key.Render("Tab")); b.WriteString(" next tab\n")
	b.WriteString(key.Render("Enter")); b.WriteString(" select/send\n\n")
	b.WriteString(title.Render("ACTIVE SESSION")); b.WriteString("\n\n")
	if s := m.ActiveSession(); s != nil { b.WriteString(s.Title) } else { b.WriteString("none") }
	return panelFixed(b.String(), width, height)
}

func (m Model) renderMainPanel(width, height int, title, key lipgloss.Style) string {
	var b strings.Builder
	b.WriteString(title.Render(strings.ToUpper(m.currentViewName()))); b.WriteString("\n\n")
	switch m.activeTab {
	case 0:
		if m.approval != nil {
			b.WriteString(title.Render("ACTION REQUEST")); b.WriteString("\n\n")
			fmt.Fprintf(&b, "Tool:    %s\nSession: %s\nCommand: %s\n", m.approval.ToolID, m.approval.SessionID, m.approval.Command)
			if m.approval.Reason != "" { fmt.Fprintf(&b, "Reason:  %s\n", m.approval.Reason) }
			b.WriteString("\n" + key.Render("[Y]") + " now   " + key.Render("[S]") + " session   " + key.Render("[A]") + " always   " + key.Render("[N]") + " deny")
		} else if len(m.messages) == 0 && len(m.toolCalls) == 0 {
			b.WriteString("No messages yet.\n\nAsk Eiksy something in the input line below.")
		} else {
			for _, message := range m.messages { fmt.Fprintf(&b, "%s: %s\n", message.Role, message.Content) }
			for _, tool := range m.toolCalls {
				fmt.Fprintf(&b, "\n[%s] %s\n", tool.Name, tool.Status)
				if tool.Output != "" { fmt.Fprintf(&b, "  %s\n", tool.Output) }
			}
		}
		b.WriteString("\n\n> "); b.WriteString(m.input)
	case 1:
		if !m.hasTerminalSession() {
			b.WriteString("No active terminal session.\n\nConnect or select a runtime session to use the terminal.")
		} else {
			fmt.Fprintf(&b, "Session: %s\n\n", m.tabs[1].Session.Title)
			if len(m.terminalLines) == 0 { b.WriteString("Connected. Ready for input.") } else {
				for _, line := range m.terminalLines { b.WriteString(line); b.WriteByte('\n') }
			}
			b.WriteString("\n\n> "); b.WriteString(m.terminalInput)
		}
	case 2:
		path := m.filePath; if path == "" { path = "." }
		fmt.Fprintf(&b, "Path: %s\n\n", path)
		if len(m.fileEntries) == 0 { b.WriteString("No files loaded yet.") } else {
			for _, entry := range m.fileEntries {
				kind := entry.Kind; if kind == "" { kind = "file" }
				fmt.Fprintf(&b, "[%-4s] %s\n", kind, entry.Name)
			}
		}
	case 3:
		b.WriteString("Tools available through the application services.\n\nUse the command palette to navigate available actions.\n\n")
		b.WriteString(key.Render("Ctrl+P")); b.WriteString("  command palette")
	case 4:
		b.WriteString("Persistent application settings.\n\n")
		lines := []string{
			fmt.Sprintf("Prompt before AI actions: %s", boolLabel(m.settings.PromptBeforeAI)),
			fmt.Sprintf("Allow cloud models:       %s", boolLabel(m.settings.AllowCloudModels)),
			fmt.Sprintf("Theme:                    %s", nonEmpty(m.settings.Theme, "default")),
		}
		for i, line := range lines {
			marker := "  "
			if i == m.settingsIndex { marker = "› " }
			b.WriteString(marker + line + "\n")
		}
		b.WriteString("\n↑/↓ select   Enter change")
	case 5:
		b.WriteString("Ask Eiksy to inspect, connect, or operate.\n\n> "); b.WriteString(m.input)
	}
	return panelFixed(b.String(), width, height)
}

func (m Model) renderPalette(width, height int, title, key lipgloss.Style) string {
	var b strings.Builder
	b.WriteString(title.Render("COMMAND PALETTE")); b.WriteString("\n\n> "); b.WriteString(m.palette.Query); b.WriteString("\n\n")
	for i, command := range m.filteredPaletteCommands() {
		marker := "  "; if i == m.palette.Selected { marker = "› " }
		fmt.Fprintf(&b, "%s%s\n", marker, command.Title)
	}
	b.WriteString("\n"); b.WriteString(key.Render("↑/↓")); b.WriteString(" select  "); b.WriteString(key.Render("Enter")); b.WriteString(" run  "); b.WriteString(key.Render("Esc")); b.WriteString(" close")
	return panelFixed(b.String(), width, height)
}

func (m Model) renderHelp(width, height int, title lipgloss.Style) string {
	var b strings.Builder
	b.WriteString(title.Render("KEYBOARD")); b.WriteString("\n\n")
	for _, line := range []string{
		"F2  Sessions       F3  Terminal",
		"F4  Files          F5  Tools",
		"F6  Tools          F7  AI          F10 Exit",
		"",
		"←/→  previous/next tab",
		"Tab  next tab      Shift+Tab previous tab",
		"↑/↓  previous/next session",
		"Enter select/send  Ctrl+N new session  Ctrl+F fork session",
		"Ctrl+P command palette",
		"Ctrl+Q quit        Esc close overlay",
	} { b.WriteString(line); b.WriteByte('\n') }
	return panelFixed(b.String(), width, height)
}

// Config contains presentation/runtime options for the terminal UI.
type Config struct {
	AltScreen   bool
	InitialView string
	Backend     Backend
}

func Run(config Config) error {
	model := NewModel().WithBackend(config.Backend)
	switch config.InitialView {
	case "sessions", "chat":
		model.activeTab = 0
	case "terminal":
		model.activeTab = 1
	case "files":
		model.activeTab = 2
	case "tools":
		model.activeTab = 3
	case "settings":
		model.activeTab = 4
	case "ai":
		model.activeTab = 5
		model.activeView = ""
	}
	options := []tea.ProgramOption{}
	if config.AltScreen {
		options = append(options, tea.WithAltScreen())
	}
	_, err := tea.NewProgram(model, options...).Run()
	return err
}
