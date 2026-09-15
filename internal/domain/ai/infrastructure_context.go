package ai

// InfrastructureContext contains non-secret runtime information that may be
// supplied to the AI as operational context. Credential material and session
// options are intentionally excluded.
type InfrastructureContext struct {
	ActiveSession *SessionContext `json:"activeSession,omitempty"`
}

type SessionContext struct {
	ID          string   `json:"id"`
	Name        string   `json:"name,omitempty"`
	ProtocolID  string   `json:"protocolId,omitempty"`
	Host        string   `json:"host,omitempty"`
	Port        int      `json:"port,omitempty"`
	Username    string   `json:"username,omitempty"`
	Status      string   `json:"status,omitempty"`
	Description string   `json:"description,omitempty"`
	Group       string   `json:"group,omitempty"`
	Tags        []string `json:"tags,omitempty"`
	CurrentDir  string   `json:"currentDir,omitempty"`
}
