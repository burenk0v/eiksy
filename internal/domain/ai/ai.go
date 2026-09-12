package ai

type ProviderClass string

const (
	ProviderClassOpenAICompatible ProviderClass = "openai_compatible"
	ProviderClassLocalOpenAI      ProviderClass = "local_openai"
)

type ProviderDescriptor struct {
	ID         string        `json:"id"`
	Name       string        `json:"name"`
	Class      ProviderClass `json:"class"`
	Model      string        `json:"model"`
	Endpoint   string        `json:"endpoint,omitempty"`
	DownloadURL string       `json:"downloadUrl,omitempty"`
	LocalPath  string        `json:"localPath,omitempty"`
	Status     string        `json:"status"`
	Running    bool          `json:"running"`
	Selected   bool          `json:"selected"`
	Token      string        `json:"-"`
	HasToken   bool          `json:"hasToken"`
	Configured bool          `json:"configured"`
}

type ContextPolicy struct {
	SendTerminalSelection bool `json:"sendTerminalSelection"`
	SendRecentOutput      bool `json:"sendRecentOutput"`
	RequireConfirmation   bool `json:"requireConfirmation"`
}

type ChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type WorkspaceState struct {
	Providers     []ProviderDescriptor `json:"providers"`
	ContextPolicy ContextPolicy        `json:"contextPolicy"`
	Messages      []ChatMessage        `json:"messages"`
	ChatSessionID string               `json:"chatSessionId"`
}
