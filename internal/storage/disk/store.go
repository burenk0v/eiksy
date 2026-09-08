package disk

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"opsy/internal/domain/ai"
	"opsy/internal/domain/credentials"
	"opsy/internal/domain/protocols"
	"opsy/internal/domain/sessions"
	"opsy/internal/domain/settings"
	"opsy/internal/domain/workspace"
	"opsy/internal/llm"
	"opsy/internal/securestorage"
)

const maxLaunchHistoryEntries = 100

type Store struct {
	mu                  sync.RWMutex
	baseDir             string
	sessionsPath        string
	settingsPath        string
	protocols           []protocols.Descriptor
	sessionProfiles     map[string]sessions.Profile
	sessionOrder        []string
	runtimeTabs         map[string]workspace.Tab
	runtimeOrder        []string
	launchHistory       []sessions.HistoryEntry
	credentialProviders []credentials.ProviderDescriptor
	aiState             ai.WorkspaceState
	settings            settings.AppSettings
	workspaceLayout     workspace.Layout
	events              []workspace.Event
	eventCounter        int64
	secretManager       *securestorage.Manager
}

type persistedSettings struct {
	settings.AppSettings
	AIState *persistedAIWorkspaceState `json:"aiState,omitempty"`
}

type persistedAIWorkspaceState struct {
	Providers     []persistedAIProviderDescriptor `json:"providers"`
	ContextPolicy ai.ContextPolicy                `json:"contextPolicy"`
	Messages      []ai.ChatMessage                `json:"messages"`
	ChatSessionID string                          `json:"chatSessionId"`
}

type persistedAIProviderDescriptor struct {
	ID         string           `json:"id"`
	Name       string           `json:"name"`
	Class      ai.ProviderClass `json:"class"`
	Model      string           `json:"model"`
	Endpoint   string           `json:"endpoint,omitempty"`
	LocalPath  string           `json:"localPath,omitempty"`
	Command    string           `json:"command,omitempty"`
	Status     string           `json:"status"`
	Selected   bool             `json:"selected"`
	Configured bool             `json:"configured"`
}

func NewStore() (*Store, error) {
	baseDir, err := llm.OpsyDir()
	if err != nil {
		return nil, err
	}
	return NewStoreAt(baseDir)
}

func NewStoreAt(baseDir string) (*Store, error) {
	return NewStoreAtWithKeyring(baseDir, nil)
}

func NewStoreAtWithKeyring(baseDir string, keyring securestorage.Keyring) (*Store, error) {
	if err := os.MkdirAll(filepath.Join(baseDir, "models"), 0o755); err != nil {
		return nil, fmt.Errorf("create opsy directory: %w", err)
	}

	var (
		secretManager *securestorage.Manager
		err           error
	)
	if keyring == nil {
		secretManager, err = securestorage.New(filepath.Join(baseDir, "secrets.db"))
	} else {
		secretManager, err = securestorage.NewWithKeyring(filepath.Join(baseDir, "secrets.db"), keyring)
	}
	if err != nil {
		return nil, err
	}

	store := &Store{
		baseDir:             baseDir,
		sessionsPath:        filepath.Join(baseDir, "sessions.json"),
		settingsPath:        filepath.Join(baseDir, "settings.json"),
		protocols:           defaultProtocols(),
		sessionProfiles:     map[string]sessions.Profile{},
		sessionOrder:        []string{},
		runtimeTabs:         map[string]workspace.Tab{},
		runtimeOrder:        []string{},
		launchHistory:       []sessions.HistoryEntry{},
		credentialProviders: []credentials.ProviderDescriptor{},
		aiState:             defaultAIState(),
		settings:            defaultSettings(),
		workspaceLayout:     defaultWorkspaceLayout(),
		events:              []workspace.Event{},
		secretManager:       secretManager,
	}

	if err := store.ensureFiles(); err != nil {
		return nil, err
	}
	if err := store.loadSessionProfiles(); err != nil {
		return nil, err
	}
	if err := store.loadSettings(); err != nil {
		return nil, err
	}

	return store, nil
}

func (s *Store) Protocols() []protocols.Descriptor {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return append([]protocols.Descriptor(nil), s.protocols...)
}

func (s *Store) SessionProfiles() []sessions.Profile {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]sessions.Profile, 0, len(s.sessionOrder))
	for _, id := range s.sessionOrder {
		result = append(result, cloneProfile(s.sessionProfiles[id]))
	}
	return result
}

func (s *Store) SessionProfile(profileID string) (sessions.Profile, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	profile, ok := s.sessionProfiles[profileID]
	return cloneProfile(profile), ok
}

func (s *Store) LaunchHistory() []sessions.HistoryEntry {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := append([]sessions.HistoryEntry(nil), s.launchHistory...)
	for left, right := 0, len(result)-1; left < right; left, right = left+1, right-1 {
		result[left], result[right] = result[right], result[left]
	}
	return result
}

func (s *Store) CredentialProviders() []credentials.ProviderDescriptor {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return append([]credentials.ProviderDescriptor(nil), s.credentialProviders...)
}

func (s *Store) AIState() ai.WorkspaceState {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return cloneAIState(s.aiState)
}

func (s *Store) Settings() settings.AppSettings {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.settings
}

func (s *Store) WorkspaceLayout() workspace.Layout {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return cloneWorkspaceLayout(s.workspaceLayout)
}

func (s *Store) RuntimeTabs() []workspace.Tab {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make([]workspace.Tab, 0, len(s.runtimeOrder))
	for _, id := range s.runtimeOrder {
		result = append(result, s.runtimeTabs[id])
	}
	return result
}

func (s *Store) Events() []workspace.Event {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make([]workspace.Event, 0, len(s.events))
	for i := len(s.events) - 1; i >= 0; i-- {
		result = append(result, s.events[i])
	}
	return result
}

func (s *Store) UpdateAIState(state ai.WorkspaceState) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.aiState = cloneAIState(state)
	for i := range s.aiState.Providers {
		s.aiState.Providers[i].Token = ""
	}
	_ = s.saveSettings()
}

func (s *Store) UpdateSettings(updated settings.AppSettings) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	updated.VaultToken = ""
	updated.VaultPassword = ""
	updated.KeePassPassword = ""
	updated.HasVaultToken = s.secretManager.SecretExists(securestorage.VaultTokenKey())
	updated.HasKeePassPassword = s.secretManager.SecretExists(securestorage.KeePassPasswordKey())
	updated.HasVaultPassword = s.secretManager.SecretExists(securestorage.VaultPasswordKey())
	s.settings = updated
	return s.saveSettings()
}

func (s *Store) OpenRuntimeTab(tab workspace.Tab) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.runtimeTabs[tab.ID]; !ok {
		s.runtimeOrder = append(s.runtimeOrder, tab.ID)
	}
	s.runtimeTabs[tab.ID] = tab
	s.workspaceLayout.ActiveTabID = tab.ID
	s.events = append(s.events, workspace.Event{
		ID:      s.nextEventIDLocked(),
		Type:    "session.opened",
		Subject: tab.Title,
		At:      time.Now().UTC().Format(time.RFC3339),
	})
}

func (s *Store) CloseRuntimeTab(sessionID string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.runtimeTabs[sessionID]; !ok {
		return false
	}
	delete(s.runtimeTabs, sessionID)
	removedIndex := -1
	filtered := make([]string, 0, len(s.runtimeOrder))
	for index, id := range s.runtimeOrder {
		if id != sessionID {
			filtered = append(filtered, id)
		} else {
			removedIndex = index
		}
	}
	s.runtimeOrder = filtered
	s.events = append(s.events, workspace.Event{
		ID:      s.nextEventIDLocked(),
		Type:    "session.closed",
		Subject: sessionID,
		At:      time.Now().UTC().Format(time.RFC3339),
	})
	if len(s.runtimeOrder) == 0 {
		s.workspaceLayout.ActiveTabID = ""
		return true
	}
	nextIndex := removedIndex
	if nextIndex >= len(s.runtimeOrder) {
		nextIndex = len(s.runtimeOrder) - 1
	}
	if nextIndex < 0 {
		nextIndex = 0
	}
	s.workspaceLayout.ActiveTabID = s.runtimeOrder[nextIndex]
	return true
}

func (s *Store) RecordLaunch(profileID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	profile, ok := s.sessionProfiles[profileID]
	if !ok {
		return
	}
	now := time.Now().UTC().Format(time.RFC3339)
	profile.LastLaunchedAt = now
	s.sessionProfiles[profile.ID] = profile
	s.launchHistory = append(s.launchHistory, sessions.HistoryEntry{ProfileID: profile.ID, ProfileName: profile.Name, LaunchedAt: now})
	if len(s.launchHistory) > maxLaunchHistoryEntries {
		s.launchHistory = append([]sessions.HistoryEntry(nil), s.launchHistory[len(s.launchHistory)-maxLaunchHistoryEntries:]...)
	}
	_ = s.saveSessionProfilesLocked()
}

func (s *Store) UpsertSessionProfile(profile sessions.Profile) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	profile = cloneProfile(profile)
	if err := s.persistProfileSecrets(profile); err != nil {
		return err
	}
	profile.Password = ""
	profile.KeyPassphrase = ""
	profile.HasPassword = s.secretManager.SecretExists(securestorage.SessionPasswordKey(profile.ID))
	profile.HasKeyPassphrase = s.secretManager.SecretExists(securestorage.SessionKeyPassphraseKey(profile.ID))
	if _, ok := s.sessionProfiles[profile.ID]; !ok {
		s.sessionOrder = append(s.sessionOrder, profile.ID)
	}
	s.sessionProfiles[profile.ID] = profile
	return s.saveSessionProfilesLocked()
}

func (s *Store) DeleteSessionProfile(profileID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.sessionProfiles[profileID]; !ok {
		return fmt.Errorf("session profile %q not found", profileID)
	}
	delete(s.sessionProfiles, profileID)
	_ = s.secretManager.DeleteSecret(securestorage.SessionPasswordKey(profileID))
	_ = s.secretManager.DeleteSecret(securestorage.SessionKeyPassphraseKey(profileID))
	filtered := make([]string, 0, len(s.sessionOrder))
	for _, id := range s.sessionOrder {
		if id != profileID {
			filtered = append(filtered, id)
		}
	}
	s.sessionOrder = filtered
	return s.saveSessionProfilesLocked()
}

func (s *Store) ensureFiles() error {
	if err := ensureJSONFile(s.sessionsPath, []byte("[]\n")); err != nil {
		return fmt.Errorf("create sessions file: %w", err)
	}
	settingsBytes, err := json.MarshalIndent(newPersistedSettings(defaultSettings(), defaultAIState()), "", "  ")
	if err != nil {
		return fmt.Errorf("marshal default settings: %w", err)
	}
	settingsBytes = append(settingsBytes, '\n')
	if err := ensureJSONFile(s.settingsPath, settingsBytes); err != nil {
		return fmt.Errorf("create settings file: %w", err)
	}
	return nil
}

func (s *Store) loadSessionProfiles() error {
	content, err := os.ReadFile(s.sessionsPath)
	if err != nil {
		return fmt.Errorf("read sessions file: %w", err)
	}
	if len(strings.TrimSpace(string(content))) == 0 {
		return nil
	}
	var profiles []sessions.Profile
	if err := json.Unmarshal(content, &profiles); err != nil {
		if renameErr := os.Rename(s.sessionsPath, s.sessionsPath+".fail"); renameErr != nil {
			return fmt.Errorf("decode sessions file: %w (also failed to rename: %v)", err, renameErr)
		}
		if writeErr := os.WriteFile(s.sessionsPath, []byte("[]\n"), 0o600); writeErr != nil {
			return fmt.Errorf("decode sessions file: %w (also failed to recreate: %v)", err, writeErr)
		}
		return nil
	}
	for _, profile := range profiles {
		profile = cloneProfile(profile)
		profile.HasPassword = s.secretManager.SecretExists(securestorage.SessionPasswordKey(profile.ID))
		profile.HasKeyPassphrase = s.secretManager.SecretExists(securestorage.SessionKeyPassphraseKey(profile.ID))
		s.sessionProfiles[profile.ID] = profile
		s.sessionOrder = append(s.sessionOrder, profile.ID)
	}
	return nil
}

func (s *Store) loadSettings() error {
	content, err := os.ReadFile(s.settingsPath)
	if err != nil {
		return fmt.Errorf("read settings file: %w", err)
	}
	if len(strings.TrimSpace(string(content))) == 0 {
		s.settings = defaultSettings()
		return s.saveSettings()
	}
	var loaded settings.AppSettings
	var persisted persistedSettings
	if err := json.Unmarshal(content, &persisted); err != nil {
		if renameErr := os.Rename(s.settingsPath, s.settingsPath+".fail"); renameErr != nil {
			return fmt.Errorf("decode settings file: %w (also failed to rename: %v)", err, renameErr)
		}
		s.settings = defaultSettings()
		s.aiState = defaultAIState()
		return s.saveSettings()
	}
	loaded = persisted.AppSettings
	defaults := defaultSettings()
	if loaded.Theme == "" {
		loaded.Theme = defaults.Theme
	}
	if loaded.DefaultProtocol == "" {
		loaded.DefaultProtocol = defaults.DefaultProtocol
	}
	if loaded.WindowLayout.SidebarWidth == 0 {
		loaded.WindowLayout.SidebarWidth = defaults.WindowLayout.SidebarWidth
	}
	if loaded.WindowLayout.AssistantWidth == 0 {
		loaded.WindowLayout.AssistantWidth = defaults.WindowLayout.AssistantWidth
	}
	if loaded.VaultMountPoint == "" {
		loaded.VaultMountPoint = defaults.VaultMountPoint
	}
	if strings.TrimSpace(loaded.VaultProvider) == "" {
		loaded.VaultProvider = defaults.VaultProvider
	}
	if loaded.VaultProvider != "vault" && loaded.VaultProvider != "keepass" {
		loaded.VaultProvider = defaults.VaultProvider
	}
	switch strings.ToLower(strings.TrimSpace(loaded.VaultAuthMethod)) {
	case "oidc", "oidc-sec", "domain":
		loaded.VaultAuthMethod = strings.ToLower(strings.TrimSpace(loaded.VaultAuthMethod))
	default:
		loaded.VaultAuthMethod = settings.DefaultVaultAuthMethod
	}
	loaded.VaultLogin = strings.TrimSpace(loaded.VaultLogin)
	loaded.KeePassDatabasePath = strings.TrimSpace(loaded.KeePassDatabasePath)
	loaded.KeePassPassword = ""
	loaded.VaultToken = ""
	loaded.VaultPassword = ""
	loaded.HasVaultToken = s.secretManager.SecretExists(securestorage.VaultTokenKey())
	loaded.HasKeePassPassword = s.secretManager.SecretExists(securestorage.KeePassPasswordKey())
	loaded.HasVaultPassword = s.secretManager.SecretExists(securestorage.VaultPasswordKey())
	loaded.PortForwardRules = normalizePortForwardRules(loaded.PortForwardRules)
	s.settings = loaded
	if persisted.AIState != nil {
		s.aiState = aiStateFromPersisted(*persisted.AIState)
	}
	return nil
}

func (s *Store) saveSessionProfilesLocked() error {
	profiles := make([]sessions.Profile, 0, len(s.sessionOrder))
	for _, id := range s.sessionOrder {
		profiles = append(profiles, s.sessionProfiles[id])
	}
	data, err := json.MarshalIndent(profiles, "", "  ")
	if err != nil {
		return fmt.Errorf("encode sessions file: %w", err)
	}
	data = append(data, '\n')
	if err := os.WriteFile(s.sessionsPath, data, 0o600); err != nil {
		return fmt.Errorf("write sessions file: %w", err)
	}
	return nil
}

func (s *Store) saveSettings() error {
	data, err := json.MarshalIndent(newPersistedSettings(s.settings, s.aiState), "", "  ")
	if err != nil {
		return fmt.Errorf("encode settings file: %w", err)
	}
	data = append(data, '\n')
	if err := os.WriteFile(s.settingsPath, data, 0o600); err != nil {
		return fmt.Errorf("write settings file: %w", err)
	}
	return nil
}

func (s *Store) nextEventIDLocked() string {
	s.eventCounter++
	return fmt.Sprintf("event-%d", s.eventCounter)
}

func ensureJSONFile(path string, defaultContent []byte) error {
	if _, err := os.Stat(path); err == nil {
		return nil
	} else if !os.IsNotExist(err) {
		return err
	}
	return os.WriteFile(path, defaultContent, 0o600)
}

func (s *Store) SecureStorageStatus() securestorage.Status {
	if s.secretManager == nil {
		return securestorage.Status{}
	}
	return s.secretManager.Status()
}

func (s *Store) EnsureMasterPassword(password string) error {
	if s.secretManager == nil {
		return fmt.Errorf("secure storage is unavailable")
	}
	err := s.secretManager.EnsureMasterPassword(password)
	if err != nil {
		return err
	}
	s.mu.Lock()
	s.settings.HasVaultToken = s.secretManager.SecretExists(securestorage.VaultTokenKey())
	s.settings.HasKeePassPassword = s.secretManager.SecretExists(securestorage.KeePassPasswordKey())
	s.settings.HasVaultPassword = s.secretManager.SecretExists(securestorage.VaultPasswordKey())
	for id, profile := range s.sessionProfiles {
		profile.HasPassword = s.secretManager.SecretExists(securestorage.SessionPasswordKey(id))
		profile.HasKeyPassphrase = s.secretManager.SecretExists(securestorage.SessionKeyPassphraseKey(id))
		s.sessionProfiles[id] = profile
	}
	defer s.mu.Unlock()
	return nil
}

func (s *Store) SecretExists(key string) bool {
	return s.secretManager != nil && s.secretManager.SecretExists(key)
}

func (s *Store) LoadSecret(key string) (string, error) {
	if s.secretManager == nil {
		return "", fmt.Errorf("secure storage is unavailable")
	}
	return s.secretManager.LoadSecret(key)
}

func (s *Store) StoreSecret(key, value string) error {
	if s.secretManager == nil {
		return fmt.Errorf("secure storage is unavailable")
	}
	return s.secretManager.StoreSecret(key, value)
}

func (s *Store) DeleteSecret(key string) error {
	if s.secretManager == nil {
		return nil
	}
	return s.secretManager.DeleteSecret(key)
}

func (s *Store) persistProfileSecrets(profile sessions.Profile) error {
	authMethod := "password"
	if profile.Options != nil && strings.TrimSpace(profile.Options["auth_method"]) != "" {
		authMethod = strings.ToLower(strings.TrimSpace(profile.Options["auth_method"]))
	}
	if strings.TrimSpace(string(profile.Password)) != "" {
		if err := s.secretManager.StoreSecret(securestorage.SessionPasswordKey(profile.ID), strings.TrimSpace(string(profile.Password))); err != nil {
			return err
		}
	}
	if strings.TrimSpace(string(profile.KeyPassphrase)) != "" {
		if err := s.secretManager.StoreSecret(securestorage.SessionKeyPassphraseKey(profile.ID), strings.TrimSpace(string(profile.KeyPassphrase))); err != nil {
			return err
		}
	}
	if authMethod == "key" {
		if err := s.secretManager.DeleteSecret(securestorage.SessionPasswordKey(profile.ID)); err != nil {
			return err
		}
	} else {
		if err := s.secretManager.DeleteSecret(securestorage.SessionKeyPassphraseKey(profile.ID)); err != nil {
			return err
		}
	}
	return nil
}

func defaultProtocols() []protocols.Descriptor {
	return []protocols.Descriptor{
		{ID: "ssh", Name: "Secure Shell", Scheme: "ssh", Capabilities: []protocols.Capability{protocols.CapabilityTerminal, protocols.CapabilityCredentialLink}},
		{ID: "sftp", Name: "SSH File Transfer", Scheme: "sftp", Capabilities: []protocols.Capability{protocols.CapabilityFileBrowser, protocols.CapabilityCredentialLink}},
		{ID: "rdp", Name: "Remote Desktop", Scheme: "rdp", Capabilities: []protocols.Capability{protocols.CapabilityDesktopDisplay, protocols.CapabilityCredentialLink}},
	}
}

func defaultAIState() ai.WorkspaceState {
	return ai.WorkspaceState{
		Providers: []ai.ProviderDescriptor{
			{ID: "openai-compatible-cloud", Name: "OpenAI-compatible Cloud", Class: ai.ProviderClassOpenAICompatible, Model: "Remote model", Status: "configuration required", Selected: true, Configured: false},
		},
		ContextPolicy: ai.ContextPolicy{SendTerminalSelection: true, SendRecentOutput: false, RequireConfirmation: true},
		Messages:      []ai.ChatMessage{{Role: "assistant", Content: "Ask for command suggestions or paste terminal errors for analysis."}},
		ChatSessionID: fmt.Sprintf("chat-%d", time.Now().UTC().UnixNano()),
	}
}

func defaultSettings() settings.AppSettings {
	return settings.AppSettings{
		Theme:               "dark",
		DefaultProtocol:     "ssh",
		WindowLayout:        settings.WindowLayout{SidebarWidth: 300, AssistantWidth: 360},
		PromptBeforeAI:      true,
		AllowCloudModels:    true,
		SSHConfigAutoLoaded: false,
		VaultMountPoint:     settings.DefaultVaultMountPoint,
		VaultAutoRenewToken: false,
		VaultAuthMethod:     settings.DefaultVaultAuthMethod,
		VaultProvider:       settings.DefaultVaultProvider,
	}
}

func defaultWorkspaceLayout() workspace.Layout {
	return workspace.Layout{SidebarSections: []workspace.SidebarSection{{ID: "sessions", Title: "Sessions"}, {ID: "files", Title: "SFTP Browser"}}}
}

func cloneAIState(state ai.WorkspaceState) ai.WorkspaceState {
	cloned := state
	cloned.Providers = append([]ai.ProviderDescriptor(nil), state.Providers...)
	cloned.Messages = append([]ai.ChatMessage{}, state.Messages...)
	return cloned
}

func cloneWorkspaceLayout(layout workspace.Layout) workspace.Layout {
	cloned := layout
	cloned.SidebarSections = append([]workspace.SidebarSection(nil), layout.SidebarSections...)
	return cloned
}

func cloneProfile(profile sessions.Profile) sessions.Profile {
	cloned := profile
	cloned.Tags = append([]string{}, profile.Tags...)
	if profile.Options != nil {
		cloned.Options = make(map[string]string, len(profile.Options))
		for key, value := range profile.Options {
			cloned.Options[key] = value
		}
	}
	cloned.Password = sessions.EncryptedString(strings.TrimSpace(string(profile.Password)))
	cloned.KeyPassphrase = sessions.EncryptedString(strings.TrimSpace(string(profile.KeyPassphrase)))
	return cloned
}

func newPersistedSettings(app settings.AppSettings, state ai.WorkspaceState) persistedSettings {
	persisted := persistedSettings{
		AppSettings: app,
	}
	persisted.AppSettings.VaultToken = ""
	persisted.AppSettings.VaultPassword = ""
	persisted.AppSettings.KeePassPassword = ""
	persisted.AIState = aiStateToPersisted(state)
	return persisted
}

func normalizePortForwardRules(rules []settings.PortForwardRule) []settings.PortForwardRule {
	normalized := make([]settings.PortForwardRule, 0, len(rules))
	for _, rule := range rules {
		entry := rule
		entry.HostID = strings.TrimSpace(entry.HostID)
		entry.LocalPort = strings.TrimSpace(entry.LocalPort)
		entry.RemoteHost = strings.TrimSpace(entry.RemoteHost)
		entry.RemotePort = strings.TrimSpace(entry.RemotePort)
		if entry.LocalPort == "" && strings.TrimSpace(entry.Ports) != "" {
			entry.LocalPort = strings.TrimSpace(entry.Ports)
		}
		normalized = append(normalized, entry)
	}
	return normalized
}

func aiStateToPersisted(state ai.WorkspaceState) *persistedAIWorkspaceState {
	persisted := &persistedAIWorkspaceState{
		ContextPolicy: state.ContextPolicy,
		Messages:      append([]ai.ChatMessage(nil), state.Messages...),
		ChatSessionID: state.ChatSessionID,
		Providers:     make([]persistedAIProviderDescriptor, 0, len(state.Providers)),
	}
	for _, provider := range state.Providers {
		persisted.Providers = append(persisted.Providers, persistedAIProviderDescriptor{
			ID:         provider.ID,
			Name:       provider.Name,
			Class:      provider.Class,
			Model:      provider.Model,
			Endpoint:   provider.Endpoint,
			LocalPath:  provider.LocalPath,
			Command:    provider.Command,
			Status:     provider.Status,
			Selected:   provider.Selected,
			Configured: provider.Configured,
		})
	}
	return persisted
}

func aiStateFromPersisted(persisted persistedAIWorkspaceState) ai.WorkspaceState {
	state := ai.WorkspaceState{
		ContextPolicy: persisted.ContextPolicy,
		Messages:      append([]ai.ChatMessage(nil), persisted.Messages...),
		ChatSessionID: persisted.ChatSessionID,
		Providers:     make([]ai.ProviderDescriptor, 0, len(persisted.Providers)),
	}
	for _, provider := range persisted.Providers {
		state.Providers = append(state.Providers, ai.ProviderDescriptor{
			ID:         provider.ID,
			Name:       provider.Name,
			Class:      provider.Class,
			Model:      provider.Model,
			Endpoint:   provider.Endpoint,
			LocalPath:  provider.LocalPath,
			Command:    provider.Command,
			Status:     provider.Status,
			Selected:   provider.Selected,
			Configured: provider.Configured,
		})
	}
	return state
}
