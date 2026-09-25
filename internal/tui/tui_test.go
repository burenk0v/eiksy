package tui

import (
	"strings"
	"testing"

	agentai "eiksy/internal/ai"
	appservice "eiksy/internal/app"
	domainai "eiksy/internal/domain/ai"
	domainsettings "eiksy/internal/domain/settings"
	sftpdomain "eiksy/internal/domain/sftp"
	domainsessions "eiksy/internal/domain/sessions"
	tea "github.com/charmbracelet/bubbletea"
)

func TestModelHandlesWindowSize(t *testing.T) {
	m := NewModel()
	next, cmd := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	if cmd != nil {
		t.Fatal("expected no command")
	}

	got := next.(Model)
	if got.width != 120 || got.height != 40 {
		t.Fatalf("expected 120x40, got %dx%d", got.width, got.height)
	}
}

func TestModelDoesNotQuitOnQ(t *testing.T) {
	m := NewModel()
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	if cmd != nil {
		t.Fatal("q must remain ordinary input")
	}
}

func TestModelQuitsOnCtrlQ(t *testing.T) {
	m := NewModel()
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyCtrlQ})
	if cmd == nil {
		t.Fatal("expected quit command")
	}
}

func TestModelHasInitialTabs(t *testing.T) {
	m := NewModel()
	want := []string{"Sessions", "Terminal", "Files", "Tools", "Settings", "AI"}
	if len(m.tabs) != len(want) {
		t.Fatalf("expected %d tabs, got %d", len(want), len(m.tabs))
	}
	for i, title := range want {
		if m.tabs[i].Title != title {
			t.Fatalf("expected tab %d to be %q, got %q", i, title, m.tabs[i].Title)
		}
	}
	if m.activeTab != 0 {
		t.Fatalf("expected initial tab 0, got %d", m.activeTab)
	}
}

func TestModelNavigatesTabs(t *testing.T) {
	m := NewModel()

	next, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRight})
	if cmd != nil {
		t.Fatal("expected no command")
	}
	m = next.(Model)
	if m.activeTab != 1 {
		t.Fatalf("expected active tab 1, got %d", m.activeTab)
	}

	next, _ = m.Update(tea.KeyMsg{Type: tea.KeyLeft})
	m = next.(Model)
	if m.activeTab != 0 {
		t.Fatalf("expected active tab 0, got %d", m.activeTab)
	}

	next, _ = m.Update(tea.KeyMsg{Type: tea.KeyLeft})
	m = next.(Model)
	if m.activeTab != len(m.tabs)-1 {
		t.Fatalf("expected wrap to last tab, got %d", m.activeTab)
	}
}

func TestModelViewContainsTabs(t *testing.T) {
	m := NewModel()
	view := m.View()
	for _, want := range []string{"EIKSY", "Think. Connect. Operate.", "Sessions", "Terminal", "Files", "Tools", "F2 Sessions", "F10 exit"} {
		if !strings.Contains(view, want) {
			t.Fatalf("expected view to contain %q, got %q", want, view)
		}
	}
}

func TestTabSessionReference(t *testing.T) {
	tab := Tab{Title: "Chat"}

	tab.BindSession("session-1", "server-01")
	if tab.Session == nil {
		t.Fatal("expected session reference")
	}
	if tab.Session.ID != "session-1" || tab.Session.Title != "server-01" {
		t.Fatalf("unexpected session reference: %+v", tab.Session)
	}

	tab.UnbindSession()
	if tab.Session != nil {
		t.Fatal("expected session reference to be cleared")
	}
}

func TestTabBindSessionIgnoresEmptyID(t *testing.T) {
	tab := Tab{Title: "Chat"}
	tab.BindSession("  ", "server-01")
	if tab.Session != nil {
		t.Fatal("expected empty session ID not to create a reference")
	}
}

func TestModelChatInput(t *testing.T) {
	m := NewModel()
	for _, r := range []rune("hello") {
		next, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
		if cmd != nil {
			t.Fatal("expected no command")
		}
		m = next.(Model)
	}
	if m.input != "hello" {
		t.Fatalf("expected input %q, got %q", "hello", m.input)
	}

	// Printable h/l characters must be treated as chat input, not navigation.

	next, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd != nil {
		t.Fatal("expected no command")
	}
	m = next.(Model)
	if m.input != "" {
		t.Fatalf("expected input to clear, got %q", m.input)
	}
	if len(m.messages) != 1 || m.messages[0].Role != "You" || m.messages[0].Content != "hello" {
		t.Fatalf("unexpected messages: %+v", m.messages)
	}
	if !strings.Contains(m.View(), "You: hello") {
		t.Fatal("expected submitted message in chat view")
	}
}

func TestModelChatIgnoresEmptyInput(t *testing.T) {
	m := NewModel()
	m.input = "   "
	m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if len(m.messages) != 0 {
		t.Fatal("expected empty input not to create a message")
	}
}


func TestModelRendersAgentToolEvents(t *testing.T) {
	m := NewModel()

	next, cmd := m.Update(agentai.Event{Type: agentai.EventToolStarted, Tool: "ssh.exec"})
	if cmd != nil {
		t.Fatal("expected no command")
	}
	m = next.(Model)

	next, _ = m.Update(agentai.Event{Type: agentai.EventToolOutput, Content: "systemctl status nginx"})
	m = next.(Model)

	next, _ = m.Update(agentai.Event{Type: agentai.EventToolFinished, Tool: "ssh.exec"})
	m = next.(Model)

	view := m.View()
	for _, want := range []string{"[ssh.exec] finished", "systemctl status nginx"} {
		if !strings.Contains(view, want) {
			t.Fatalf("expected view to contain %q, got %q", want, view)
		}
	}
	if len(m.messages) != 0 {
		t.Fatal("tool events must not create chat messages")
	}
}

func TestModelRendersAgentErrorsAndCancellation(t *testing.T) {
	m := NewModel()

	next, _ := m.Update(agentai.Event{Type: agentai.EventError, Err: testError("provider unavailable")})
	m = next.(Model)
	next, _ = m.Update(agentai.Event{Type: agentai.EventCancellation})
	m = next.(Model)

	view := m.View()
	for _, want := range []string{"System: provider unavailable", "System: AI request cancelled"} {
		if !strings.Contains(view, want) {
			t.Fatalf("expected view to contain %q, got %q", want, view)
		}
	}
}

type testError string

func (e testError) Error() string { return string(e) }


type testApprovalResolver struct {
	requestID string
	mode      string
	err       error
}

func (r *testApprovalResolver) ResolveApproval(requestID, mode string) error {
	r.requestID = requestID
	r.mode = mode
	return r.err
}

func TestModelRendersAndResolvesApproval(t *testing.T) {
	resolver := &testApprovalResolver{}
	m := NewModel().WithApprovalResolver(resolver)

	next, cmd := m.Update(agentai.Event{
		Type: agentai.EventApprovalRequired,
		Tool: "ssh.exec",
		Approval: &agentai.ApprovalRequest{
			RequestID: "cmdreq-1",
			ToolID:    "shell",
			SessionID: "session-1",
			Command:   "systemctl restart nginx",
			Reason:    "change",
		},
	})
	if cmd != nil {
		t.Fatal("expected no command while rendering approval")
	}
	m = next.(Model)

	view := m.View()
	for _, want := range []string{
		"ACTION REQUEST",
		"systemctl restart nginx",
		"change",
		"[Y] now",
		"[S] session",
		"[A] always",
		"[N] deny",
	} {
		if !strings.Contains(view, want) {
			t.Fatalf("expected approval view to contain %q, got %q", want, view)
		}
	}

	next, cmd = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'y'}})
	if cmd == nil {
		t.Fatal("expected approval command")
	}
	m = next.(Model)
	if m.approval != nil {
		t.Fatal("expected approval prompt to clear immediately")
	}
	result := cmd()
	if result == nil {
		t.Fatal("expected approval result message")
	}
	next, _ = m.Update(result)
	m = next.(Model)
	if resolver.requestID != "cmdreq-1" || resolver.mode != "now" {
		t.Fatalf("unexpected approval resolution: request=%q mode=%q", resolver.requestID, resolver.mode)
	}
	if !strings.Contains(m.View(), "Approval now applied.") {
		t.Fatal("expected approval result in chat")
	}
}

func TestModelDeniesApprovalWithoutResolver(t *testing.T) {
	m := NewModel()
	next, _ := m.Update(agentai.Event{
		Type: agentai.EventApprovalRequired,
		Approval: &agentai.ApprovalRequest{
			RequestID: "cmdreq-2",
			ToolID:    "shell",
			SessionID: "session-1",
			Command:   "uname -a",
		},
	})
	m = next.(Model)
	next, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'n'}})
	if cmd != nil {
		t.Fatal("expected local fallback without resolver")
	}
	m = next.(Model)
	if m.approval != nil {
		t.Fatal("expected approval prompt to clear")
	}
	if !strings.Contains(m.View(), "Approval unavailable for cmdreq-2.") {
		t.Fatal("expected denial fallback message")
	}
}


func TestModelChatInputStillSubmitsWithSessionBrowser(t *testing.T) {
	m := NewModel().WithChatSessions([]SessionRef{{ID: "chat-1", Title: "API tests"}})
	m.input = "hello"
	next, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd != nil {
		t.Fatal("expected chat submission to remain local")
	}
	m = next.(Model)
	if len(m.messages) != 1 || m.messages[0].Content != "hello" {
		t.Fatalf("expected chat message, got %+v", m.messages)
	}
}

func TestModelSessionBrowserNavigation(t *testing.T) {
	m := NewModel().WithChatSessions([]SessionRef{
		{ID: "chat-1", Title: "API tests"},
		{ID: "chat-2", Title: "Production debug"},
		{ID: "chat-3", Title: "Deploy"},
	})

	if got := m.ActiveSession(); got == nil || got.ID != "chat-1" {
		t.Fatalf("expected first active session, got %+v", got)
	}
	if m.tabs[0].Session == nil || m.tabs[0].Session.ID != "chat-1" {
		t.Fatalf("expected chat tab to reference first session: %+v", m.tabs[0].Session)
	}

	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyDown})
	m = next.(Model)
	if got := m.ActiveSession(); got == nil || got.ID != "chat-2" {
		t.Fatalf("expected second active session, got %+v", got)
	}
	next, _ = m.Update(tea.KeyMsg{Type: tea.KeyUp})
	m = next.(Model)
	if got := m.ActiveSession(); got == nil || got.ID != "chat-1" {
		t.Fatalf("expected navigation back to first session, got %+v", got)
	}
	next, _ = m.Update(tea.KeyMsg{Type: tea.KeyUp})
	m = next.(Model)
	if got := m.ActiveSession(); got == nil || got.ID != "chat-3" {
		t.Fatalf("expected session navigation to wrap, got %+v", got)
	}
}

type runtimeTestBackend struct {
	profiles []domainsessions.Profile
	launched appservice.RuntimeSessionView
	connected []string
	disconnected []string
	closed []string
	sent []string
}

func (b *runtimeTestBackend) ListChatSessions() []domainai.ChatSession { return nil }
func (b *runtimeTestBackend) CreateChatSession(string) (domainai.ChatSession, error) { return domainai.ChatSession{}, nil }
func (b *runtimeTestBackend) SelectChatSession(string) error { return nil }
func (b *runtimeTestBackend) ForkChatSession(string, string) (domainai.ChatSession, error) { return domainai.ChatSession{}, nil }
func (b *runtimeTestBackend) GetSettings() domainsettings.AppSettings { return domainsettings.AppSettings{} }
func (b *runtimeTestBackend) UpdateSettings(domainsettings.AppSettings) error { return nil }
func (b *runtimeTestBackend) ListSessionProfiles() []domainsessions.Profile { return append([]domainsessions.Profile(nil), b.profiles...) }
func (b *runtimeTestBackend) CreateSessionProfileInput(domainsessions.ProfileInput) error { return nil }
func (b *runtimeTestBackend) DeleteSessionProfile(string) error { return nil }
func (b *runtimeTestBackend) LaunchSession(profileID string) (appservice.RuntimeSessionView, error) {
	b.launched = appservice.RuntimeSessionView{ID:"ssh-1",Title:"prod",ProtocolID:"ssh",ProfileID:profileID,Status:"connecting"}
	return b.launched, nil
}
func (b *runtimeTestBackend) ConnectSession(sessionID string) error { b.connected = append(b.connected, sessionID); return nil }
func (b *runtimeTestBackend) ReconnectSession(sessionID string) error { b.connected = append(b.connected, "reconnect:"+sessionID); return nil }
func (b *runtimeTestBackend) DisconnectSession(sessionID string) error { b.disconnected = append(b.disconnected, sessionID); return nil }
func (b *runtimeTestBackend) CloseSession(sessionID string) error { b.closed = append(b.closed, sessionID); return nil }
func (b *runtimeTestBackend) SendSSHInput(sessionID, data string) error { b.sent = append(b.sent, sessionID+":"+data); return nil }
func (b *runtimeTestBackend) AcceptSSHHostKey(string) error { return nil }

type sftpRuntimeTestBackend struct {
	runtimeTestBackend
	listed []string
	read []string
}

func (b *sftpRuntimeTestBackend) ListSFTPFiles(sessionID, targetPath string) ([]sftpdomain.FileEntry, error) {
	b.listed = append(b.listed, sessionID+":"+targetPath)
	return []sftpdomain.FileEntry{
		{Name:"config", Path:"/etc/config", IsDir:true},
		{Name:"readme.txt", Path:"/etc/readme.txt", IsDir:false},
	}, nil
}

func (b *sftpRuntimeTestBackend) NavigateSFTP(sessionID, targetPath string) ([]sftpdomain.FileEntry, error) {
	return b.ListSFTPFiles(sessionID, targetPath)
}

func (b *sftpRuntimeTestBackend) ReadSFTPFile(sessionID, filePath string) (string, error) {
	b.read = append(b.read, sessionID+":"+filePath)
	return "hello from remote", nil
}

func TestModelSFTPFilesBrowseAndRead(t *testing.T) {
	backend := &sftpRuntimeTestBackend{runtimeTestBackend: runtimeTestBackend{
		profiles: []domainsessions.Profile{{ID:"prod",Name:"prod",ProtocolID:"ssh",Host:"host",Port:22,Username:"ops"}},
	}}
	m := NewModel().WithBackend(backend).WithTerminalSession("ssh-1", "prod")
	m.activeTab = 2

	next, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil { t.Fatal("expected SFTP listing command") }
	m = next.(Model)
	result := cmd()
	next, _ = m.Update(result)
	m = next.(Model)
	if len(backend.listed) != 1 || backend.listed[0] != "ssh-1:" { t.Fatalf("unexpected SFTP list calls: %v", backend.listed) }
	if len(m.sftpEntries) != 2 || m.sftpSelected != 0 { t.Fatalf("unexpected SFTP entries: %+v selected=%d", m.sftpEntries, m.sftpSelected) }

	next, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	m = next.(Model)
	if m.sftpSelected != 1 { t.Fatalf("expected second file selected, got %d", m.sftpSelected) }

	next, cmd = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil { t.Fatal("expected SFTP read command") }
	m = next.(Model)
	result = cmd()
	next, _ = m.Update(result)
	m = next.(Model)
	if len(backend.read) != 1 || backend.read[0] != "ssh-1:/etc/readme.txt" { t.Fatalf("unexpected SFTP read calls: %v", backend.read) }
	if m.fileContent != "hello from remote" { t.Fatalf("unexpected file content: %q", m.fileContent) }
	if !strings.Contains(m.View(), "FILE: /etc/readme.txt") || !strings.Contains(m.View(), "hello from remote") { t.Fatal("expected remote file content in Files view") }
}

func TestModelSFTPDirectoryNavigation(t *testing.T) {
	backend := &sftpRuntimeTestBackend{runtimeTestBackend: runtimeTestBackend{}}
	m := NewModel().WithBackend(backend).WithTerminalSession("ssh-1", "prod")
	m.activeTab = 2
	m.sftpEntries = []sftpdomain.FileEntry{{Name:"config",Path:"/etc/config",IsDir:true}}
	next, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil { t.Fatal("expected directory navigation command") }
	_ = next
	result := cmd()
	next, _ = m.Update(result)
	m = next.(Model)
	if m.sftpPath != "/etc/config" { t.Fatalf("expected directory path, got %q", m.sftpPath) }
	if len(backend.listed) != 1 || backend.listed[0] != "ssh-1:/etc/config" { t.Fatalf("unexpected directory list calls: %v", backend.listed) }
}

func TestModelRuntimeSessionLifecycle(t *testing.T) {
	backend := &runtimeTestBackend{profiles: []domainsessions.Profile{{ID:"prod",Name:"prod",ProtocolID:"ssh",Host:"host",Port:22,Username:"ops"}}}
	m := NewModel().WithBackend(backend)

	next, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil { t.Fatal("expected launch command") }
	m = next.(Model)
	result := cmd()
	if result == nil { t.Fatal("expected launch result") }
	next, cmd = m.Update(result)
	m = next.(Model)
	if cmd == nil { t.Fatal("expected connect command") }
	result = cmd()
	next, _ = m.Update(result)
	m = next.(Model)

	if len(backend.connected) != 1 || backend.connected[0] != "ssh-1" { t.Fatalf("expected ssh connection, got %v", backend.connected) }
	if m.tabs[1].Session == nil || m.tabs[1].Session.ID != "ssh-1" { t.Fatalf("expected terminal binding, got %+v", m.tabs[1].Session) }

	m.terminalInput = "ls"
	next, cmd = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil { t.Fatal("expected terminal send command") }
	_ = cmd()
	m = next.(Model)
	if len(backend.sent) != 1 || backend.sent[0] != "ssh-1:ls\n" { t.Fatalf("unexpected terminal input: %v", backend.sent) }

	next, cmd = m.Update(tea.KeyMsg{Type: tea.KeyCtrlX})
	if cmd == nil { t.Fatal("expected disconnect command") }
	m = next.(Model)
	_ = cmd()
	if len(backend.disconnected) != 1 || backend.disconnected[0] != "ssh-1" { t.Fatalf("unexpected disconnects: %v", backend.disconnected) }

	next, cmd = m.Update(tea.KeyMsg{Type: tea.KeyCtrlW})
	if cmd == nil { t.Fatal("expected close command") }
	m = next.(Model)
	_ = cmd()
	if len(backend.closed) != 1 || backend.closed[0] != "ssh-1" { t.Fatalf("unexpected closes: %v", backend.closed) }
}

type testSessionSelector struct {
	sessionID string
	err       error
}

func (s *testSessionSelector) SelectChatSession(sessionID string) error {
	s.sessionID = sessionID
	return s.err
}

func TestModelSessionBrowserSelectsThroughApplicationService(t *testing.T) {
	selector := &testSessionSelector{}
	m := NewModel().
		WithChatSessions([]SessionRef{{ID: "chat-1", Title: "API tests"}}).
		WithSessionSelector(selector)

	next, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("expected session selection command")
	}
	m = next.(Model)
	if selector.sessionID != "" {
		t.Fatal("selection should be deferred to the command")
	}

	result := cmd()
	if result == nil {
		t.Fatal("expected session selection result")
	}
	next, _ = m.Update(result)
	m = next.(Model)
	if selector.sessionID != "chat-1" {
		t.Fatalf("expected application service to receive chat-1, got %q", selector.sessionID)
	}
	if !strings.Contains(m.View(), "Session chat-1 selected.") {
		t.Fatal("expected selection confirmation")
	}
}

func TestModelSessionBrowserView(t *testing.T) {
	m := NewModel().WithChatSessions([]SessionRef{
		{ID: "chat-1", Title: "API tests"},
		{ID: "chat-2", Title: "Production debug"},
	})
	view := m.View()
	for _, want := range []string{"Sessions", "Session 1/2", "ACTIVE SESSION", "API tests"} {
		if !strings.Contains(view, want) {
			t.Fatalf("expected view to contain %q, got %q", want, view)
		}
	}
}

type testSessionForker struct {
	sessionID string
	title     string
	result    SessionRef
	err       error
}

func (f *testSessionForker) ForkChatSession(sessionID, title string) (domainai.ChatSession, error) {
	f.sessionID = sessionID
	f.title = title
	return domainai.ChatSession{ID: f.result.ID, Title: f.result.Title}, f.err
}

func TestModelForksActiveSessionThroughApplicationService(t *testing.T) {
	forker := &testSessionForker{result: SessionRef{ID: "chat-2", Title: "API tests (fork)"}}
	m := NewModel().
		WithChatSessions([]SessionRef{{ID: "chat-1", Title: "API tests"}}).
		WithSessionForker(forker)

	next, cmd := m.Update(tea.KeyMsg{Type: tea.KeyCtrlF})
	if cmd == nil { t.Fatal("expected fork command") }
	m = next.(Model)
	result := cmd()
	if result == nil { t.Fatal("expected fork result") }
	next, _ = m.Update(result)
	m = next.(Model)

	if forker.sessionID != "chat-1" || forker.title != "API tests (fork)" {
		t.Fatalf("unexpected fork request: id=%q title=%q", forker.sessionID, forker.title)
	}
	if len(m.sessions) != 2 || m.activeSession != 1 {
		t.Fatalf("expected fork to become active session: %+v active=%d", m.sessions, m.activeSession)
	}
	if m.tabs[0].Session == nil || m.tabs[0].Session.ID != "chat-2" {
		t.Fatalf("expected chat tab to follow fork: %+v", m.tabs[0].Session)
	}
	if !strings.Contains(m.View(), "Session API tests (fork) forked.") {
		t.Fatal("expected fork confirmation")
	}
}

func TestModelForkUnavailable(t *testing.T) {
	m := NewModel().WithChatSessions([]SessionRef{{ID: "chat-1", Title: "API tests"}})
	next, cmd := m.Update(tea.KeyMsg{Type: tea.KeyCtrlF})
	if cmd != nil { t.Fatal("expected local fallback") }
	m = next.(Model)
	if !strings.Contains(m.View(), "Session forking unavailable.") {
		t.Fatal("expected unavailable message")
	}
}


func TestModelCommandPaletteOpensFiltersAndCloses(t *testing.T) {
	m := NewModel()
	next, cmd := m.Update(tea.KeyMsg{Type: tea.KeyCtrlP})
	if cmd != nil { t.Fatal("expected no command") }
	m = next.(Model)
	if m.palette == nil { t.Fatal("expected command palette to open") }
	if !strings.Contains(m.View(), "COMMAND PALETTE") || !strings.Contains(m.View(), "Fork active session") {
		t.Fatal("expected command palette contents")
	}

	next, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'f'}})
	m = next.(Model)
	if m.palette.Query != "f" || !strings.Contains(m.View(), "Fork active session") {
		t.Fatalf("expected palette query filtering, got %+v", m.palette)
	}

	next, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m = next.(Model)
	if m.palette != nil { t.Fatal("expected palette to close") }
}

func TestModelCommandPaletteRunsNavigationCommand(t *testing.T) {
	m := NewModel()
	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyCtrlP})
	m = next.(Model)

	next, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'n'}})
	if cmd != nil { t.Fatal("expected no command while filtering") }
	m = next.(Model)
	if !strings.Contains(m.View(), "Next tab") {
		t.Fatal("expected next tab command in filtered palette")
	}

	next, cmd = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd != nil { t.Fatal("expected navigation command to execute locally") }
	m = next.(Model)
	if m.palette != nil || m.activeTab != 1 {
		t.Fatalf("expected palette closed and next tab selected: palette=%+v tab=%d", m.palette, m.activeTab)
	}
}

func TestModelCommandPaletteQuit(t *testing.T) {
	m := NewModel()
	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyCtrlP})
	m = next.(Model)
	next, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	m = next.(Model)
	if cmd != nil { t.Fatal("expected no command while filtering") }
	next, cmd = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil { t.Fatal("expected quit command from palette") }
	m = next.(Model)
	if m.palette != nil { t.Fatal("expected palette to close") }
}


func TestModelNavigationShortcuts(t *testing.T) {
	m := NewModel()
	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyShiftTab})
	m = next.(Model)
	if m.activeTab != 5 {
		t.Fatalf("expected shift+tab to select previous tab, got %d", m.activeTab)
	}
	next, _ = m.Update(tea.KeyMsg{Type: tea.KeyTab})
	m = next.(Model)
	if m.activeTab != 0 {
		t.Fatalf("expected tab to select next tab, got %d", m.activeTab)
	}
}

func TestModelQuitShortcutDoesNotConsumeChatInput(t *testing.T) {
	m := NewModel()
	for _, r := range []rune{'h', 'q', 'u', 'i', 't'} {
		next, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
		if cmd != nil {
			t.Fatalf("expected chat input, got quit command for %q", r)
		}
		m = next.(Model)
	}
	if m.input != "hquit" {
		t.Fatalf("expected q to remain chat input when input is non-empty, got %q", m.input)
	}
}

func TestModelShortcutHelpOpensAndCloses(t *testing.T) {
	m := NewModel()
	next, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'?'}})
	if cmd != nil {
		t.Fatal("expected no command")
	}
	m = next.(Model)
	if !m.shortcuts {
		t.Fatal("expected shortcut help to open")
	}
	view := m.View()
	for _, want := range []string{"KEYBOARD", "F2", "F3", "F4", "F5", "F6", "F7", "F10", "Ctrl+Q"} {
		if !strings.Contains(view, want) {
			t.Fatalf("expected shortcut help to contain %q, got %q", want, view)
		}
	}
	next, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m = next.(Model)
	if m.shortcuts {
		t.Fatal("expected shortcut help to close")
	}
}

func TestModelTerminalViewAcceptsInput(t *testing.T) {
	m := NewModel().WithTerminalSession("runtime-1", "Production")

	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyRight})
	m = next.(Model)
	if m.activeTab != 1 {
		t.Fatalf("expected terminal tab, got %d", m.activeTab)
	}

	for _, r := range []rune("qecho status") {
		next, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
		if cmd != nil {
			t.Fatalf("expected terminal input, got command for %q", r)
		}
		m = next.(Model)
	}

	if m.terminalInput != "qecho status" {
		t.Fatalf("expected terminal input to be preserved, got %q", m.terminalInput)
	}

	next, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd != nil {
		t.Fatal("expected local terminal submission")
	}
	m = next.(Model)

	if m.terminalInput != "" {
		t.Fatalf("expected terminal input to clear, got %q", m.terminalInput)
	}
	if len(m.terminalLines) != 1 || m.terminalLines[0] != "$ qecho status" {
		t.Fatalf("unexpected terminal lines: %+v", m.terminalLines)
	}
	if !strings.Contains(m.View(), "Terminal") || !strings.Contains(m.View(), "$ qecho status") {
		t.Fatalf("expected terminal content in view, got %q", m.View())
	}
}

func TestModelTerminalViewShowsActiveSession(t *testing.T) {
	m := NewModel().WithTerminalSession("runtime-1", "Production")
	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyRight})
	m = next.(Model)

	view := m.View()
	for _, want := range []string{"Terminal", "Session: Production", "Connected. Ready for input."} {
		if !strings.Contains(view, want) {
			t.Fatalf("expected terminal view to contain %q, got %q", want, view)
		}
	}
}


func TestModelFilesViewShowsEmptyStateAndSession(t *testing.T) {
	m := NewModel().WithChatSessions([]SessionRef{{ID: "chat-1", Title: "Production"}})
	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyRight})
	m = next.(Model)
	next, _ = m.Update(tea.KeyMsg{Type: tea.KeyRight})
	m = next.(Model)

	view := m.View()
	for _, want := range []string{"Files", "Session 1/1", "Path: .", "No files loaded yet."} {
		if !strings.Contains(view, want) {
			t.Fatalf("expected files view to contain %q, got %q", want, view)
		}
	}
}

func TestModelFilesViewRendersApplicationProvidedEntries(t *testing.T) {
	m := NewModel().WithFileEntries("/etc/eiksy", []FileEntry{
		{Name: "config.yaml", Kind: "file"},
		{Name: "sessions", Kind: "dir"},
	})
	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyRight})
	m = next.(Model)
	next, _ = m.Update(tea.KeyMsg{Type: tea.KeyRight})
	m = next.(Model)

	view := m.View()
	for _, want := range []string{"Path: /etc/eiksy", "[file] config.yaml", "[dir ] sessions"} {
		if !strings.Contains(view, want) {
			t.Fatalf("expected files view to contain %q, got %q", want, view)
		}
	}
}


func TestModelViewShowsContextStatus(t *testing.T) {
	m := NewModel().WithChatSessions([]SessionRef{{ID: "chat-1", Title: "Production"}, {ID: "chat-2", Title: "Deploy"}})
	view := m.View()
	for _, want := range []string{"Sessions", "Session 1/2"} {
		if !strings.Contains(view, want) {
			t.Fatalf("expected view to contain %q, got %q", want, view)
		}
	}
}


func TestModelTerminalDoesNotAcceptInputWithoutRuntimeSession(t *testing.T) {
	m := NewModel()
	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyRight})
	m = next.(Model)

	for _, r := range []rune("echo status") {
		next, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
		if cmd != nil {
			t.Fatal("expected no command")
		}
		m = next.(Model)
	}

	if m.terminalInput != "" {
		t.Fatalf("terminal must remain read-only without a runtime session, got %q", m.terminalInput)
	}
	if !strings.Contains(m.View(), "No active terminal session.") {
		t.Fatal("expected terminal empty state")
	}
}

func TestModelTerminalBindsRuntimeSessionSeparatelyFromChatSessions(t *testing.T) {
	m := NewModel().
		WithChatSessions([]SessionRef{{ID: "chat-1", Title: "Conversation"}}).
		WithTerminalSession("runtime-1", "Production")

	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyRight})
	m = next.(Model)

	if !m.hasTerminalSession() || m.tabs[1].Session.ID != "runtime-1" {
		t.Fatalf("expected runtime terminal session, got %+v", m.tabs[1].Session)
	}
	if m.tabs[0].Session == nil || m.tabs[0].Session.ID != "chat-1" {
		t.Fatalf("expected chat session to remain separate, got %+v", m.tabs[0].Session)
	}
}


func TestModelFunctionKeyNavigation(t *testing.T) {
	m := NewModel()
	cases := []struct {
		key  tea.KeyType
		want int
		view string
	}{
		{tea.KeyF2, 0, "Sessions"},
		{tea.KeyF3, 1, "Terminal"},
		{tea.KeyF4, 2, "Files"},
		{tea.KeyF5, 3, "Tools"},
		{tea.KeyF6, 4, "Settings"},
		{tea.KeyF7, 5, "AI"},
	}
	for _, tc := range cases {
		next, cmd := m.Update(tea.KeyMsg{Type: tc.key})
		if cmd != nil {
			t.Fatalf("%s: expected no command", tc.view)
		}
		m = next.(Model)
		if m.activeTab != tc.want {
			t.Fatalf("%s: expected tab %d, got %d", tc.view, tc.want, m.activeTab)
		}
		if !strings.Contains(m.View(), tc.view) {
			t.Fatalf("%s: expected view %q", tc.view, m.View())
		}
	}
	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyF7})
	m = next.(Model)
	if m.activeTab != 5 || !strings.Contains(m.View(), "AI") {
		t.Fatalf("expected AI mode, got view=%q", m.View())
	}
	next, cmd := m.Update(tea.KeyMsg{Type: tea.KeyF10})
	if cmd == nil {
		t.Fatal("expected F10 to quit")
	}
}
