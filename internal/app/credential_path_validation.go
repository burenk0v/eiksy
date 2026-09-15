package app

import (
	"fmt"
	"path"
	"strings"
)

// ValidateCredentialPath verifies that a configured Vault/KeePass secret path
// resolves to an existing leaf secret without exposing the secret value.
func (s *Service) ValidateCredentialPath(provider, secretPath string) error {
	provider = strings.ToLower(strings.TrimSpace(provider))
	if provider != "vault" && provider != "keepass" {
		return fmt.Errorf("unsupported credential provider %q", provider)
	}

	normalized, err := normalizeCredentialPath(secretPath)
	if err != nil {
		return err
	}

	exists, err := s.credentialPathExistsForProvider(provider, normalized)
	if err != nil {
		return fmt.Errorf("validate %s credential path %q: %w", provider, normalized, err)
	}
	if !exists {
		return fmt.Errorf("credential path %q was not found", normalized)
	}
	return nil
}

func normalizeCredentialPath(value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", fmt.Errorf("credential path is required")
	}
	if strings.ContainsRune(value, '\x00') {
		return "", fmt.Errorf("credential path contains NUL byte")
	}
	value = strings.Trim(value, "/")
	if value == "" || value == "." {
		return "", fmt.Errorf("credential path must identify a secret")
	}
	for _, segment := range strings.Split(value, "/") {
		if segment == ".." {
			return "", fmt.Errorf("credential path must not escape its provider root")
		}
	}
	cleaned := path.Clean(value)
	if cleaned == "." || cleaned == ".." || strings.HasPrefix(cleaned, "../") {
		return "", fmt.Errorf("credential path must not escape its provider root")
	}
	return cleaned, nil
}
