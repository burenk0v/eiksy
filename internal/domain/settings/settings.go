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

type AppSettings struct {
	Theme            string       `json:"theme"`
	DefaultProtocol  string       `json:"defaultProtocol"`
	WindowLayout     WindowLayout `json:"windowLayout"`
	PromptBeforeAI   bool         `json:"promptBeforeAi"`
	AllowCloudModels bool         `json:"allowCloudModels"`
	LogLevel         LogLevel     `json:"logLevel"`
	ShowLogPanel     bool         `json:"showLogPanel"`
	SaveLogsToFile   bool         `json:"saveLogsToFile"`
	LogRotationSize  int          `json:"logRotationSize"`
	VaultAddress     string       `json:"vaultAddress"`
	VaultMountPoint  string       `json:"vaultMountPoint"`
	VaultToken       string       `json:"-"`
}
