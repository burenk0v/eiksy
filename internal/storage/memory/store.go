package memory

import (
	"fmt"
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
	now := time.Now().UTC()
	lastSSH := now.Add(-45 * time.Minute)
	lastRDP := now.Add(-6 * time.Hour)

	store := &Store{
		protocols: []protocols.Descriptor{
			{ID: "ssh", Name: "Secure Shell", Scheme: "ssh", Capabilities: []protocols.Capability{protocols.CapabilityTerminal, protocols.CapabilityCredentialLink}},
			{ID: "sftp", Name: "SSH File Transfer", Scheme: "sftp", Capabilities: []protocols.Capability{protocols.CapabilityFileBrowser, protocols.CapabilityCredentialLink}},
			{ID: "rdp", Name: "Remote Desktop", Scheme: "rdp", Capabilities: []protocols.Capability{protocols.CapabilityDesktopDisplay, protocols.CapabilityCredentialLink}},
		},
		sessionProfiles: map[string]sessions.Profile{
			"ops-linux-admin": {
				ID:             "ops-linux-admin",
				Name:           "ops-linux-admin",
				Group:          "Production",
				Tags:           []string{"linux", "ssh", "critical"},
				Favorite:       true,
				ProtocolID:     "ssh",
				Host:           "prod-shell.internal",
				Port:           22,
				Username:       "ops",
				SecretRef:      "vault:kv/platform/prod-shell#password",
				LastLaunchedAt: lastSSH.Format(time.RFC3339),
			},
			"artifact-mirror": {
				ID:             "artifact-mirror",
				Name:           "artifact-mirror",
				Group:          "Shared Services",
				Tags:           []string{"sftp", "artifacts"},
				Favorite:       false,
				ProtocolID:     "sftp",
				Host:           "mirror.internal",
				Port:           22,
				Username:       "mirrorbot",
				SecretRef:      "keepass:Infrastructure/Mirror",
				LastLaunchedAt: lastSSH.Add(-2 * time.Hour).Format(time.RFC3339),
			},
			"helpdesk-rdp": {
				ID:             "helpdesk-rdp",
				Name:           "helpdesk-rdp",
				Group:          "Support",
				Tags:           []string{"windows", "rdp"},
				Favorite:       true,
				ProtocolID:     "rdp",
				Host:           "helpdesk-gateway.internal",
				Port:           3389,
				Username:       "support",
				SecretRef:      "wincred:helpdesk-rdp",
				LastLaunchedAt: lastRDP.Format(time.RFC3339),
			},
		},
		sessionOrder: []string{"ops-linux-admin", "artifact-mirror", "helpdesk-rdp"},
		runtimeTabs: map[string]workspace.Tab{
			"session-1": {
				ID:          "session-1",
				Title:       "ops-linux-admin",
				ProtocolID:  "ssh",
				ProfileID:   "ops-linux-admin",
				Status:      "connected",
				Description: "Production / ssh / ops@prod-shell.internal:22",
			},
		},
		runtimeOrder: []string{"session-1"},
		launchHistory: []sessions.HistoryEntry{
			{ProfileID: "helpdesk-rdp", ProfileName: "helpdesk-rdp", LaunchedAt: lastRDP.Format(time.RFC3339)},
			{ProfileID: "ops-linux-admin", ProfileName: "ops-linux-admin", LaunchedAt: lastSSH.Format(time.RFC3339)},
		},
		credentialProviders: []credentials.ProviderDescriptor{
			{
				ID:   "vault-primary",
				Name: "HashiCorp Vault",
				Type: credentials.ProviderTypeVault,
				Capabilities: []credentials.Capability{
					credentials.CapabilityBrowseSecrets,
					credentials.CapabilityRefreshToken,
				},
				Status: credentials.AuthStatus{
					State:         "authenticated",
					ExpiresAt:     now.Add(45 * time.Minute).Format(time.RFC3339),
					Renewable:     true,
					Authenticated: true,
				},
			},
			{
				ID:   "keepass-local",
				Name: "KeePass",
				Type: credentials.ProviderTypeKeePass,
				Capabilities: []credentials.Capability{
					credentials.CapabilityBrowseSecrets,
				},
				Status: credentials.AuthStatus{
					State:         "locked",
					Renewable:     false,
					Authenticated: false,
				},
			},
			{
				ID:   "windows-vault",
				Name: "Windows Password Manager",
				Type: credentials.ProviderTypeWindows,
				Capabilities: []credentials.Capability{
					credentials.CapabilityBrowseSecrets,
				},
				Status: credentials.AuthStatus{
					State:         "ready",
					Renewable:     false,
					Authenticated: true,
				},
			},
		},
		aiState: ai.WorkspaceState{
			Providers: []ai.ProviderDescriptor{
				{ID: "llama-cpp-local", Name: "Qwen3 8B Local", Class: ai.ProviderClassLocal, Model: "Qwen3 8B (Q4_K_M)", Status: "download required", Selected: true, Configured: false},
				{ID: "openai-compatible-cloud", Name: "OpenAI-compatible Cloud", Class: ai.ProviderClassOpenAICompatible, Model: "Remote model", Status: "token required", Endpoint: "https://api.example.internal/v1", Configured: false},
			},
			ContextPolicy: ai.ContextPolicy{
				SendTerminalSelection: true,
				SendRecentOutput:      false,
				RequireConfirmation:   true,
			},
			Messages: []ai.ChatMessage{
				{Role: "assistant", Content: "Ask for command suggestions or paste terminal errors for analysis."},
			},
		},
		settings: settings.AppSettings{
			Theme:           "dark",
			DefaultProtocol: "ssh",
			WindowLayout: settings.WindowLayout{
				SidebarWidth:   320,
				AssistantWidth: 360,
			},
			PromptBeforeAI:   true,
			AllowCloudModels: true,
		},
		workspaceLayout: workspace.Layout{
			SidebarSections: []workspace.SidebarSection{
				{ID: "sessions", Title: "Sessions"},
				{ID: "files", Title: "SFTP Browser"},
				{ID: "credentials", Title: "Credentials"},
			},
			ActiveTabID: "session-1",
		},
		events: []workspace.Event{
			{ID: "event-1", Type: "session.opened", Subject: "ops-linux-admin", At: lastSSH.Format(time.RFC3339)},
			{ID: "event-2", Type: "vault.token.renewal_scheduled", Subject: "vault-primary", At: now.Format(time.RFC3339)},
		},
		eventCounter: 2,
	}

	return store
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
		result = append(result, s.sessionProfiles[id])
	}

	return result
}

func (s *Store) SessionProfile(profileID string) (sessions.Profile, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	profile, ok := s.sessionProfiles[profileID]
	return profile, ok
}

func (s *Store) UpsertSessionProfile(profile sessions.Profile) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.sessionProfiles[profile.ID]; !ok {
		s.sessionOrder = append(s.sessionOrder, profile.ID)
	}

	s.sessionProfiles[profile.ID] = profile
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

func (s *Store) Settings() settings.AppSettings {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.settings
}

func (s *Store) WorkspaceLayout() workspace.Layout {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.workspaceLayout
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

func cloneAIState(state ai.WorkspaceState) ai.WorkspaceState {
	cloned := state
	cloned.Providers = append([]ai.ProviderDescriptor(nil), state.Providers...)
	cloned.Messages = append([]ai.ChatMessage(nil), state.Messages...)
	return cloned
}
