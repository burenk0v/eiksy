package memory

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"opsy/internal/domain/ai"
	"opsy/internal/domain/credentials"
	"opsy/internal/domain/protocols"
	"opsy/internal/domain/sessions"
	"opsy/internal/domain/settings"
	"opsy/internal/domain/workspace"
)

const maxLaunchHistoryEntries = 100

type Store struct {
	mu                  sync.RWMutex
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
}

func NewStore() *Store {
	return &Store{
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
	}
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

func (s *Store) UpsertSessionProfile(profile sessions.Profile) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	profile = cloneProfile(profile)
	if _, ok := s.sessionProfiles[profile.ID]; !ok {
		s.sessionOrder = append(s.sessionOrder, profile.ID)
	}

	s.sessionProfiles[profile.ID] = profile
	return nil
}

func (s *Store) DeleteSessionProfile(profileID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.sessionProfiles[profileID]; !ok {
		return fmt.Errorf("session profile %q not found", profileID)
	}

	delete(s.sessionProfiles, profileID)
	filtered := make([]string, 0, len(s.sessionOrder))
	for _, id := range s.sessionOrder {
		if id != profileID {
			filtered = append(filtered, id)
		}
	}
	s.sessionOrder = filtered
	return nil
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

func (s *Store) UpdateAIState(state ai.WorkspaceState) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.aiState = cloneAIState(state)
}

func (s *Store) UpdateSettings(updated settings.AppSettings) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.settings = updated
	return nil
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
	filteredOrder := make([]string, 0, len(s.runtimeOrder))
	for index, id := range s.runtimeOrder {
		if id != sessionID {
			filteredOrder = append(filteredOrder, id)
		} else {
			removedIndex = index
		}
	}
	if removedIndex >= 0 {
		s.events = append(s.events, workspace.Event{
			ID:      s.nextEventIDLocked(),
			Type:    "session.closed",
			Subject: sessionID,
			At:      time.Now().UTC().Format(time.RFC3339),
		})
	}
	s.runtimeOrder = filteredOrder
	if len(s.runtimeOrder) > 0 {
		nextIndex := removedIndex
		if nextIndex >= len(s.runtimeOrder) {
			nextIndex = len(s.runtimeOrder) - 1
		}
		if nextIndex < 0 {
			nextIndex = 0
		}
		s.workspaceLayout.ActiveTabID = s.runtimeOrder[nextIndex]
	} else {
		s.workspaceLayout.ActiveTabID = ""
	}

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

	s.launchHistory = append(s.launchHistory, sessions.HistoryEntry{
		ProfileID:   profile.ID,
		ProfileName: profile.Name,
		LaunchedAt:  now,
	})
	if len(s.launchHistory) > maxLaunchHistoryEntries {
		s.launchHistory = append([]sessions.HistoryEntry(nil), s.launchHistory[len(s.launchHistory)-maxLaunchHistoryEntries:]...)
	}
}

func (s *Store) nextEventIDLocked() string {
	s.eventCounter++
	return fmt.Sprintf("event-%d", s.eventCounter)
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
		ContextPolicy: ai.ContextPolicy{
			SendTerminalSelection: true,
			SendRecentOutput:      false,
			RequireConfirmation:   true,
		},
		Messages:      []ai.ChatMessage{{Role: "assistant", Content: "Ask for command suggestions or paste terminal errors for analysis."}},
		ChatSessionID: fmt.Sprintf("chat-%d", time.Now().UTC().UnixNano()),
	}
}

func defaultSettings() settings.AppSettings {
	return settings.AppSettings{
		Theme:           "dark",
		DefaultProtocol: "ssh",
		WindowLayout: settings.WindowLayout{
			SidebarWidth:   300,
			AssistantWidth: 360,
		},
		PromptBeforeAI:   true,
		AllowCloudModels: true,
		LogLevel:         settings.LogLevelInfo,
		ShowLogPanel:     false,
		SaveLogsToFile:   false,
		LogRotationSize:  10 * 1024 * 1024,
		VaultMountPoint:  "secret",
	}
}

func defaultWorkspaceLayout() workspace.Layout {
	return workspace.Layout{
		SidebarSections: []workspace.SidebarSection{
			{ID: "sessions", Title: "Sessions"},
			{ID: "files", Title: "SFTP Browser"},
		},
	}
}

func cloneAIState(state ai.WorkspaceState) ai.WorkspaceState {
	cloned := state
	cloned.Providers = append([]ai.ProviderDescriptor(nil), state.Providers...)
	cloned.Messages = append([]ai.ChatMessage(nil), state.Messages...)
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
	return cloned
}
