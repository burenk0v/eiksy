package protocols

import "context"

type Capability string

const (
	CapabilityTerminal       Capability = "terminal"
	CapabilityFileBrowser    Capability = "file_browser"
	CapabilityDesktopDisplay Capability = "desktop_display"
	CapabilityCredentialLink Capability = "credential_link"
)

type Descriptor struct {
	ID           string       `json:"id"`
	Name         string       `json:"name"`
	Scheme       string       `json:"scheme"`
	Capabilities []Capability `json:"capabilities"`
}

type ConnectionSpec struct {
	ProtocolID string            `json:"protocolId"`
	Host       string            `json:"host"`
	Port       int               `json:"port"`
	Username   string            `json:"username"`
	SecretRef  string            `json:"secretRef,omitempty"`
	Options    map[string]string `json:"options,omitempty"`
}

type RuntimeSession struct {
	ID          string `json:"id"`
	ProfileID   string `json:"profileId"`
	ProtocolID  string `json:"protocolId"`
	Status      string `json:"status"`
	DisplayName string `json:"displayName"`
}

type Provider interface {
	Descriptor() Descriptor
	Open(context.Context, ConnectionSpec) (RuntimeSession, error)
	Close(context.Context, string) error
}
