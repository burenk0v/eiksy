package resources

import "eiksy/internal/domain/protocols"

// Resource is the non-secret operational identity of a connectable target.
// Credentials, secret references and provider-specific authentication data are
// intentionally excluded from this contract.
type Resource struct {
	ID              string                 `json:"id"`
	Name            string                 `json:"name"`
	Group           string                 `json:"group,omitempty"`
	Tags            []string               `json:"tags,omitempty"`
	Kind            string                 `json:"kind"`
	ProtocolID      string                 `json:"protocolId"`
	Host            string                 `json:"host"`
	Port            int                    `json:"port"`
	Username        string                 `json:"username,omitempty"`
	Capabilities    []protocols.Capability `json:"capabilities"`
	Status          string                 `json:"status"`
	ActiveSessionID string                 `json:"activeSessionId,omitempty"`
}
