package settings

type WindowLayout struct {
	SidebarWidth   int `json:"sidebarWidth"`
	AssistantWidth int `json:"assistantWidth"`
}

const (
	DefaultVaultMountPoint = "secret"
	DefaultVaultProvider   = "vault"
	DefaultVaultAuthMethod = "token"
)

// PortForwardRule describes a single SSH local-port-forwarding rule.
type PortForwardRule struct {
	Ports      string `json:"ports"` // legacy field for backward compatibility
	LocalPort  string `json:"localPort"`
	RemoteHost string `json:"remoteHost"`
	RemotePort string `json:"remotePort"`
	HostID     string `json:"hostId"`
	Enabled    bool   `json:"enabled"`
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
	SSHConfigAutoLoaded bool              `json:"sshConfigAutoLoaded"`
	VaultAddress        string            `json:"vaultAddress"`
	VaultMountPoint     string            `json:"vaultMountPoint"`
	VaultAutoRenewToken bool              `json:"vaultAutoRenewToken"`
	VaultAuthMethod     string            `json:"vaultAuthMethod"`
	VaultLogin          string            `json:"vaultLogin"`
	VaultProvider       string            `json:"vaultProvider"`
	KeePassDatabasePath string            `json:"keepassDatabasePath"`
	KeePassPassword     string            `json:"-"`
	VaultToken          string            `json:"-"`
	VaultPassword       string            `json:"-"`
	HasKeePassPassword  bool              `json:"hasKeePassPassword"`
	HasVaultToken       bool              `json:"hasVaultToken"`
	HasVaultPassword    bool              `json:"hasVaultPassword"`
}
