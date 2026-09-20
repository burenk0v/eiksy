package credentials

type SecretReference struct {
	ProviderID string `json:"providerId"`
	Path       string `json:"path"`
	Field      string `json:"field,omitempty"`
}
