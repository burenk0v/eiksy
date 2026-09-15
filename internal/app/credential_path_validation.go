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

	parent := path.Dir(normalized)
	name := path.Base(normalized)
	if parent == "." {
		parent = ""
	}

	entries, err := s.ListVaultSecretsForProvider(provider, parent)
	if err != nil {
		return fmt.Errorf("validate %s credential path %q: %w", provider, normalized, err)
	}
	for _, entry := range entries {
		entryPath, err := normalizeCredentialPath(entry.Path)
		if err != nil || entryPath != normalized {
			continue
		}
		if path.Base(entryPath) != name {
			continue
		}
		if entry.IsDir {
			return fmt.Errorf("credential path %q points to a directory", normalized)
		}
		return nil
	}

	return fmt.Errorf("credential path %q was not found", normalized)
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
