package credentials

import "context"

type ProviderType string

const (
	ProviderTypeVault   ProviderType = "vault"
	ProviderTypeKeePass ProviderType = "keepass"
	ProviderTypeWindows ProviderType = "windows_password_manager"
)

type Capability string

const (
	CapabilityBrowseSecrets Capability = "browse_secrets"
	CapabilityRefreshToken  Capability = "refresh_token"
)

type ProviderDescriptor struct {
	ID           string       `json:"id"`
	Name         string       `json:"name"`
	Type         ProviderType `json:"type"`
	Capabilities []Capability `json:"capabilities"`
	Status       AuthStatus   `json:"status"`
}

type AuthStatus struct {
	State         string `json:"state"`
	ExpiresAt     string `json:"expiresAt,omitempty"`
	Renewable     bool   `json:"renewable"`
	Authenticated bool   `json:"authenticated"`
}

type SecretReference struct {
	ProviderID string `json:"providerId"`
	Path       string `json:"path"`
	Field      string `json:"field,omitempty"`
}

type SecretNode struct {
	ID       string `json:"id"`
	Label    string `json:"label"`
	Path     string `json:"path"`
	NodeType string `json:"nodeType"`
}

type Provider interface {
	Descriptor() ProviderDescriptor
	Browse(context.Context, string) ([]SecretNode, error)
	Renew(context.Context) (AuthStatus, error)
}
