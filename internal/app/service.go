package app

import (
	"context"
	"fmt"
	"net/url"
	"strings"
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
	store        stateStore
	localManager localModelManager
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

type localModelManager interface {
	DownloadQwen3Model(context.Context) (string, error)
	StartLocalServer(context.Context, string) (string, string, error)
}

func NewService(store stateStore, localManager localModelManager) *Service {
	return &Service{
		store:        store,
		localManager: localManager,
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
	if s.localManager == nil {
		return fmt.Errorf("local model manager is not configured")
	}

	modelPath, err := s.localManager.DownloadQwen3Model(ctx)
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

	endpoint, command, err := s.localManager.StartLocalServer(ctx, state.Providers[index].LocalPath)
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
