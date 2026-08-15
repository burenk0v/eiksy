package app

import (
	"fmt"
	"time"

	"opsy/internal/domain/ai"
	"opsy/internal/domain/credentials"
	"opsy/internal/domain/protocols"
	"opsy/internal/domain/sessions"
	"opsy/internal/domain/settings"
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
	store stateStore
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
	OpenRuntimeTab(workspace.Tab)
	CloseRuntimeTab(string) bool
	RecordLaunch(string)
}

func NewService(store stateStore) *Service {
	return &Service{store: store}
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

	return nil
}
