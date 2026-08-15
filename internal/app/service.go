package app

import (
	"context"
	"fmt"
	"net/url"
	"regexp"
	"strings"
	"time"

	"opsy/internal/domain/ai"
	"opsy/internal/domain/credentials"
	"opsy/internal/domain/protocols"
	"opsy/internal/domain/sessions"
	"opsy/internal/domain/settings"
	sftpdomain "opsy/internal/domain/sftp"
	"opsy/internal/domain/workspace"
)

type ShellState struct {
	Protocols           []protocols.Descriptor           `json:"protocols"`
	SessionProfiles     []sessions.Profile               `json:"sessionProfiles"`
	ActiveSessions      []RuntimeSessionView             `json:"activeSessions"`
	SessionHistory      []sessions.HistoryEntry          `json:"sessionHistory"`
	CredentialProviders []credentials.ProviderDescriptor `json:"credentialProviders"`
	AI                  ai.WorkspaceState                `json:"ai"`
	Workspace           WorkspaceView                    `json:"workspace"`
	Settings            settings.AppSettings             `json:"settings"`
}

type WorkspaceView struct {
	Layout       workspace.Layout  `json:"layout"`
	RecentEvents []workspace.Event `json:"recentEvents"`
}

type RuntimeSessionView struct {
	ID          string `json:"id"`
	Title       string `json:"title"`
	ProtocolID  string `json:"protocolId"`
	ProfileID   string `json:"profileId"`
	Status      string `json:"status"`
	Description string `json:"description"`
}

type Service struct {
	ctx          context.Context
	store        stateStore
	localManager localModelManager
	sshManager   sshManager
	sftpManager  sftpManager
	emitFn       func(eventName string, data ...interface{})
}

type stateStore interface {
	Protocols() []protocols.Descriptor
	SessionProfiles() []sessions.Profile
	SessionProfile(string) (sessions.Profile, bool)
	LaunchHistory() []sessions.HistoryEntry
	CredentialProviders() []credentials.ProviderDescriptor
	AIState() ai.WorkspaceState
	Settings() settings.AppSettings
	WorkspaceLayout() workspace.Layout
	RuntimeTabs() []workspace.Tab
	Events() []workspace.Event
	UpdateAIState(ai.WorkspaceState)
	OpenRuntimeTab(workspace.Tab)
	CloseRuntimeTab(string) bool
	RecordLaunch(string)
}

type sessionProfileMutator interface {
	UpsertSessionProfile(sessions.Profile) error
	DeleteSessionProfile(string) error
}

type localModelManager interface {
	DownloadQwen3Model(context.Context) (string, error)
	DownloadQwen3ModelWithProgress(context.Context, func(downloaded, total int64)) (string, error)
	StartLocalServer(context.Context, string) (string, string, error)
}

type sshManager interface {
	Connect(context.Context, string, string, int, string, string, map[string]string) error
	SendInput(string, string) error
	ResizeTerminal(string, int, int) error
	Disconnect(string) error
	SetOutputHandler(string, func(data string))
	GetCurrentDir(string) (string, error)
}

type sftpManager interface {
	Connect(context.Context, string, string, int, string, string, map[string]string) error
	Connected(string) bool
	ListDir(string, string) ([]sftpdomain.FileEntry, error)
	ReadFile(string, string) (string, error)
	WriteFile(string, string, string) error
	Disconnect(string) error
}

func NewService(store stateStore, localManager localModelManager, sshManager sshManager, sftpManager sftpManager) *Service {
	return &Service{
		store:        store,
		localManager: localManager,
		sshManager:   sshManager,
		sftpManager:  sftpManager,
		emitFn:       func(string, ...interface{}) {},
	}
}

func (s *Service) SetRuntimeContext(ctx context.Context, emitFn func(eventName string, data ...interface{})) {
	s.ctx = ctx
	if emitFn != nil {
		s.emitFn = emitFn
	}
}

func (s *Service) GetShellState() ShellState {
	tabs := s.store.RuntimeTabs()
	activeSessions := make([]RuntimeSessionView, 0, len(tabs))
	for _, tab := range tabs {
		activeSessions = append(activeSessions, RuntimeSessionView(tab))
	}

	return ShellState{
		Protocols:           s.store.Protocols(),
		SessionProfiles:     s.store.SessionProfiles(),
		ActiveSessions:      activeSessions,
		SessionHistory:      s.store.LaunchHistory(),
		CredentialProviders: s.store.CredentialProviders(),
		AI:                  s.store.AIState(),
		Workspace: WorkspaceView{
			Layout:       s.store.WorkspaceLayout(),
			RecentEvents: s.store.Events(),
		},
		Settings: s.store.Settings(),
	}
}

func (s *Service) CreateSessionProfile(profile sessions.Profile) error {
	mutator, ok := s.store.(sessionProfileMutator)
	if !ok {
		return fmt.Errorf("session profiles are read-only")
	}

	profile = normalizeProfile(profile)
	if err := s.validateProfile(profile); err != nil {
		return err
	}
	if profile.ID == "" {
		profile.ID = s.nextProfileID(profile.Name)
	}
	if existing, found := s.store.SessionProfile(profile.ID); found && profile.LastLaunchedAt == "" {
		profile.LastLaunchedAt = existing.LastLaunchedAt
	}

	return mutator.UpsertSessionProfile(profile)
}

func (s *Service) DeleteSessionProfile(id string) error {
	mutator, ok := s.store.(sessionProfileMutator)
	if !ok {
		return fmt.Errorf("session profiles are read-only")
	}
	id = strings.TrimSpace(id)
	if id == "" {
		return fmt.Errorf("session profile id is required")
	}
	return mutator.DeleteSessionProfile(id)
}

func (s *Service) LaunchSession(profileID string) (RuntimeSessionView, error) {
	profile, ok := s.store.SessionProfile(profileID)
	if !ok {
		return RuntimeSessionView{}, fmt.Errorf("session profile %q not found", profileID)
	}

	tab := workspace.Tab{
		ID:          fmt.Sprintf("%s-%d", profile.ProtocolID, time.Now().UTC().UnixNano()),
		Title:       profile.Name,
		ProtocolID:  profile.ProtocolID,
		ProfileID:   profile.ID,
		Status:      "connecting",
		Description: fmt.Sprintf("%s / %s / %s@%s:%d", profile.Group, profile.ProtocolID, profile.Username, profile.Host, profile.Port),
	}

	s.store.OpenRuntimeTab(tab)
	s.store.RecordLaunch(profile.ID)

	return RuntimeSessionView(tab), nil
}

func (s *Service) CloseSession(sessionID string) error {
	if ok := s.store.CloseRuntimeTab(sessionID); !ok {
		return fmt.Errorf("active session %q not found", sessionID)
	}
	if s.sshManager != nil {
		_ = s.sshManager.Disconnect(sessionID)
	}
	if s.sftpManager != nil {
		_ = s.sftpManager.Disconnect(sessionID)
	}
	return nil
}

func (s *Service) ConnectSSH(ctx context.Context, tabID, profileID string) error {
	if s.sshManager == nil {
		return fmt.Errorf("ssh manager is not configured")
	}
	profile, ok := s.store.SessionProfile(profileID)
	if !ok {
		return fmt.Errorf("session profile %q not found", profileID)
	}
	if profile.ProtocolID != "ssh" {
		return fmt.Errorf("session profile %q does not use ssh", profileID)
	}

	s.sshManager.SetOutputHandler(tabID, func(data string) {
		s.emitFn(fmt.Sprintf("terminal:output:%s", tabID), map[string]string{"data": data})
	})
	_ = s.updateTabStatus(tabID, "connecting")
	if err := s.sshManager.Connect(s.resolveContext(ctx), tabID, profile.Host, profile.Port, profile.Username, profile.Password, profile.Options); err != nil {
		_ = s.updateTabStatus(tabID, "error")
		return err
	}
	return s.updateTabStatus(tabID, "connected")
}

func (s *Service) SendSSHInput(tabID, data string) error {
	if s.sshManager == nil {
		return fmt.Errorf("ssh manager is not configured")
	}
	return s.sshManager.SendInput(tabID, data)
}

func (s *Service) ResizeTerminal(tabID string, cols, rows int) error {
	if s.sshManager == nil {
		return fmt.Errorf("ssh manager is not configured")
	}
	return s.sshManager.ResizeTerminal(tabID, cols, rows)
}

func (s *Service) DisconnectSSH(tabID string) error {
	if s.sshManager == nil {
		return fmt.Errorf("ssh manager is not configured")
	}
	if err := s.sshManager.Disconnect(tabID); err != nil {
		return err
	}
	return s.updateTabStatus(tabID, "disconnected")
}

func (s *Service) ListSFTPFiles(tabID, targetPath string) ([]sftpdomain.FileEntry, error) {
	if s.sftpManager == nil {
		return nil, fmt.Errorf("sftp manager is not configured")
	}
	if err := s.ensureSFTPConnection(tabID); err != nil {
		return nil, err
	}
	pathToList := strings.TrimSpace(targetPath)
	if pathToList == "" && s.sshManager != nil {
		currentDir, err := s.sshManager.GetCurrentDir(tabID)
		if err == nil && strings.TrimSpace(currentDir) != "" {
			pathToList = strings.TrimSpace(currentDir)
		}
	}
	if pathToList == "" {
		pathToList = "."
	}
	return s.sftpManager.ListDir(tabID, pathToList)
}

func (s *Service) NavigateSFTP(tabID, targetPath string) ([]sftpdomain.FileEntry, error) {
	return s.ListSFTPFiles(tabID, targetPath)
}

func (s *Service) ReadSFTPFile(tabID, filePath string) (string, error) {
	if s.sftpManager == nil {
		return "", fmt.Errorf("sftp manager is not configured")
	}
	if err := s.ensureSFTPConnection(tabID); err != nil {
		return "", err
	}
	filePath = strings.TrimSpace(filePath)
	if filePath == "" {
		return "", fmt.Errorf("file path is required")
	}
	return s.sftpManager.ReadFile(tabID, filePath)
}

func (s *Service) SaveSFTPFile(tabID, filePath, content string) error {
	if s.sftpManager == nil {
		return fmt.Errorf("sftp manager is not configured")
	}
	if err := s.ensureSFTPConnection(tabID); err != nil {
		return err
	}
	filePath = strings.TrimSpace(filePath)
	if filePath == "" {
		return fmt.Errorf("file path is required")
	}
	return s.sftpManager.WriteFile(tabID, filePath, content)
}

func (s *Service) SelectAIProvider(providerID string) error {
	state := s.store.AIState()
	index := providerIndexByID(state.Providers, providerID)
	if index < 0 {
		return fmt.Errorf("ai provider %q not found", providerID)
	}

	for i := range state.Providers {
		state.Providers[i].Selected = state.Providers[i].ID == providerID
	}

	state.Messages = appendStatusMessage(state.Messages, fmt.Sprintf("Switched AI provider to %s.", state.Providers[index].Name))
	s.store.UpdateAIState(state)
	return nil
}

func (s *Service) SaveCloudProvider(endpoint, token string) error {
	endpoint = strings.TrimSpace(endpoint)
	token = strings.TrimSpace(token)
	if endpoint == "" {
		return fmt.Errorf("cloud endpoint is required")
	}
	if token == "" {
		return fmt.Errorf("cloud token is required")
	}

	parsed, err := url.Parse(endpoint)
	if err != nil || parsed.Host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return fmt.Errorf("cloud endpoint must be a valid http or https url")
	}

	state := s.store.AIState()
	index := providerIndexByClass(state.Providers, ai.ProviderClassOpenAICompatible)
	if index < 0 {
		return fmt.Errorf("cloud ai provider is not available")
	}

	for i := range state.Providers {
		state.Providers[i].Selected = i == index
	}

	state.Providers[index].Endpoint = endpoint
	state.Providers[index].Token = token
	state.Providers[index].Status = "ready"
	state.Providers[index].Configured = true
	state.Messages = appendStatusMessage(state.Messages, fmt.Sprintf("Saved cloud AI provider %s.", state.Providers[index].Name))
	s.store.UpdateAIState(state)
	return nil
}

func (s *Service) DownloadLocalModel(ctx context.Context) error {
	return s.DownloadLocalModelWithProgress(ctx, nil)
}

func (s *Service) DownloadLocalModelWithProgress(ctx context.Context, progressFn func(downloaded, total int64)) error {
	if s.localManager == nil {
		return fmt.Errorf("local model manager is not configured")
	}

	modelPath, err := s.localManager.DownloadQwen3ModelWithProgress(s.resolveContext(ctx), progressFn)
	if err != nil {
		return err
	}

	state := s.store.AIState()
	index := providerIndexByClass(state.Providers, ai.ProviderClassLocal)
	if index < 0 {
		return fmt.Errorf("local ai provider is not available")
	}

	for i := range state.Providers {
		state.Providers[i].Selected = i == index
	}

	state.Providers[index].LocalPath = modelPath
	state.Providers[index].Status = "downloaded"
	state.Providers[index].Configured = false
	state.Messages = appendStatusMessage(state.Messages, "Downloaded the Qwen3 8B local model.")
	s.store.UpdateAIState(state)
	return nil
}

func (s *Service) StartLocalModel(ctx context.Context) error {
	if s.localManager == nil {
		return fmt.Errorf("local model manager is not configured")
	}

	state := s.store.AIState()
	index := providerIndexByClass(state.Providers, ai.ProviderClassLocal)
	if index < 0 {
		return fmt.Errorf("local ai provider is not available")
	}

	endpoint, command, err := s.localManager.StartLocalServer(s.resolveContext(ctx), state.Providers[index].LocalPath)
	if err != nil {
		return err
	}

	for i := range state.Providers {
		state.Providers[i].Selected = i == index
	}

	state.Providers[index].Endpoint = endpoint
	state.Providers[index].Command = command
	state.Providers[index].Status = "running via llama.cpp"
	state.Providers[index].Configured = true
	state.Messages = appendStatusMessage(state.Messages, "Started the Qwen3 8B local model with llama.cpp.")
	s.store.UpdateAIState(state)
	return nil
}

func (s *Service) ensureSFTPConnection(tabID string) error {
	tab, ok := s.runtimeTab(tabID)
	if !ok {
		return fmt.Errorf("active session %q not found", tabID)
	}
	profile, ok := s.store.SessionProfile(tab.ProfileID)
	if !ok {
		return fmt.Errorf("session profile %q not found", tab.ProfileID)
	}
	if s.sftpManager.Connected(tabID) {
		return nil
	}
	return s.sftpManager.Connect(s.resolveContext(nil), tabID, profile.Host, profile.Port, profile.Username, profile.Password, profile.Options)
}

func (s *Service) runtimeTab(tabID string) (workspace.Tab, bool) {
	for _, tab := range s.store.RuntimeTabs() {
		if tab.ID == tabID {
			return tab, true
		}
	}
	return workspace.Tab{}, false
}

func (s *Service) updateTabStatus(tabID, status string) error {
	tab, ok := s.runtimeTab(tabID)
	if !ok {
		return fmt.Errorf("active session %q not found", tabID)
	}
	tab.Status = status
	s.store.OpenRuntimeTab(tab)
	return nil
}

func (s *Service) validateProfile(profile sessions.Profile) error {
	if strings.TrimSpace(profile.Name) == "" {
		return fmt.Errorf("session name is required")
	}
	if strings.TrimSpace(profile.ProtocolID) == "" {
		return fmt.Errorf("protocol is required")
	}
	if strings.TrimSpace(profile.Host) == "" {
		return fmt.Errorf("host is required")
	}
	if strings.TrimSpace(profile.Username) == "" {
		return fmt.Errorf("username is required")
	}
	if profile.Port <= 0 {
		return fmt.Errorf("port must be greater than 0")
	}
	if !s.protocolExists(profile.ProtocolID) {
		return fmt.Errorf("protocol %q is not supported", profile.ProtocolID)
	}
	return nil
}

func (s *Service) protocolExists(protocolID string) bool {
	for _, protocol := range s.store.Protocols() {
		if protocol.ID == protocolID {
			return true
		}
	}
	return false
}

var nonSlugPattern = regexp.MustCompile(`[^a-z0-9]+`)

func (s *Service) nextProfileID(name string) string {
	base := strings.Trim(nonSlugPattern.ReplaceAllString(strings.ToLower(name), "-"), "-")
	if base == "" {
		base = "session"
	}
	candidate := base
	counter := 2
	for {
		if _, exists := s.store.SessionProfile(candidate); !exists {
			return candidate
		}
		candidate = fmt.Sprintf("%s-%d", base, counter)
		counter++
	}
}

func normalizeProfile(profile sessions.Profile) sessions.Profile {
	profile.ID = strings.TrimSpace(profile.ID)
	profile.Name = strings.TrimSpace(profile.Name)
	profile.Group = strings.TrimSpace(profile.Group)
	profile.ProtocolID = strings.TrimSpace(strings.ToLower(profile.ProtocolID))
	profile.Host = strings.TrimSpace(profile.Host)
	profile.Username = strings.TrimSpace(profile.Username)
	profile.Password = strings.TrimSpace(profile.Password)
	if profile.Port <= 0 {
		profile.Port = 22
	}
	profile.Tags = normalizeTags(profile.Tags)
	return profile
}

func normalizeTags(tags []string) []string {
	result := make([]string, 0, len(tags))
	seen := map[string]struct{}{}
	for _, tag := range tags {
		tag = strings.TrimSpace(tag)
		if tag == "" {
			continue
		}
		if _, ok := seen[tag]; ok {
			continue
		}
		seen[tag] = struct{}{}
		result = append(result, tag)
	}
	return result
}

func (s *Service) resolveContext(ctx context.Context) context.Context {
	if ctx != nil {
		return ctx
	}
	if s.ctx != nil {
		return s.ctx
	}
	return context.Background()
}

func providerIndexByID(providers []ai.ProviderDescriptor, providerID string) int {
	for index, provider := range providers {
		if provider.ID == providerID {
			return index
		}
	}

	return -1
}

func providerIndexByClass(providers []ai.ProviderDescriptor, class ai.ProviderClass) int {
	for index, provider := range providers {
		if provider.Class == class {
			return index
		}
	}

	return -1
}

func appendStatusMessage(messages []ai.ChatMessage, content string) []ai.ChatMessage {
	updated := append([]ai.ChatMessage(nil), messages...)
	updated = append(updated, ai.ChatMessage{
		Role:    "assistant",
		Content: content,
	})
	return updated
}
