package app

import (
	"eiksy/internal/domain/ai"
	"eiksy/internal/domain/credentials"
	"eiksy/internal/domain/protocols"
	"eiksy/internal/domain/sessions"
	"eiksy/internal/domain/settings"
	"eiksy/internal/domain/workspace"
)

// ShellState is the secret-free application snapshot exposed to the UI.
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
	ID, Title, ProtocolID, ProfileID, Status, Description string
}

// CloudProviderAuthSession is the UI-visible state of the browser auth flow.
type CloudProviderAuthSession struct {
	ID, Status, AuthURL, Token, Message, Endpoint string
}
