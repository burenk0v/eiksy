package settings

type WindowLayout struct {
	SidebarWidth   int `json:"sidebarWidth"`
	AssistantWidth int `json:"assistantWidth"`
}

// LogLevel controls which log entries are shown in the log panel.
// Supported values: "debug", "info", "warn", "error".
type LogLevel string

const (
	LogLevelDebug          LogLevel = "debug"
	LogLevelInfo           LogLevel = "info"
	LogLevelWarn           LogLevel = "warn"
	LogLevelError          LogLevel = "error"
	DefaultLogRotationSize          = 10 * 1024 * 1024
	DefaultVaultMountPoint          = "secret"
)

// PortForwardRule describes a single SSH local-port-forwarding rule.
type PortForwardRule struct {
	Ports   string `json:"ports"`
	HostID  string `json:"hostId"`
	Enabled bool   `json:"enabled"`
}

type AppSettings struct {
	Theme               string            `json:"theme"`
	DefaultProtocol     string            `json:"defaultProtocol"`
	WindowLayout        WindowLayout      `json:"windowLayout"`
	PromptBeforeAI      bool              `json:"promptBeforeAi"`
	AllowCloudModels    bool              `json:"allowCloudModels"`
	SSHForwardPorts     string            `json:"sshForwardPorts"`
	SSHForwardHostID    string            `json:"sshForwardHostId"`
	PortForwardRules    []PortForwardRule `json:"portForwardRules"`
	LogLevel            LogLevel          `json:"logLevel"`
	ShowLogPanel        bool              `json:"showLogPanel"`
	SaveLogsToFile      bool              `json:"saveLogsToFile"`
	LogRotationSize     int               `json:"logRotationSize"`
	VaultAddress        string            `json:"vaultAddress"`
	VaultMountPoint     string            `json:"vaultMountPoint"`
	VaultAutoRenewToken bool              `json:"vaultAutoRenewToken"`
	VaultToken          string            `json:"-"`
}
