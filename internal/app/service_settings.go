package app

import (
	"strings"

	"eiksy/internal/domain/settings"
	"eiksy/internal/securestorage"
)

func (s *Service) UpdateSettings(updated settings.AppSettings) error {
	current := s.store.Settings()
	// Preserve fields that are not exposed in the update call.
	if updated.Theme == "" {
		updated.Theme = current.Theme
	}
	if updated.DefaultProtocol == "" {
		updated.DefaultProtocol = current.DefaultProtocol
	}
	if updated.WindowLayout.SidebarWidth == 0 {
		updated.WindowLayout.SidebarWidth = current.WindowLayout.SidebarWidth
	}
	if updated.WindowLayout.AssistantWidth == 0 {
		updated.WindowLayout.AssistantWidth = current.WindowLayout.AssistantWidth
	}
	if strings.TrimSpace(updated.VaultMountPoint) == "" {
		updated.VaultMountPoint = current.VaultMountPoint
	}
	if strings.TrimSpace(updated.VaultMountPoint) == "" {
		updated.VaultMountPoint = settings.DefaultVaultMountPoint
	}
	if strings.TrimSpace(updated.VaultProvider) == "" {
		updated.VaultProvider = current.VaultProvider
	}
	if strings.TrimSpace(updated.VaultProvider) == "" {
		updated.VaultProvider = settings.DefaultVaultProvider
	}
	updated.VaultProvider = strings.ToLower(strings.TrimSpace(updated.VaultProvider))
	if updated.VaultProvider != "vault" && updated.VaultProvider != "keepass" {
		updated.VaultProvider = settings.DefaultVaultProvider
	}
	if strings.TrimSpace(updated.VaultAuthMethod) == "" {
		updated.VaultAuthMethod = current.VaultAuthMethod
	}
	updated.VaultAuthMethod = normalizeVaultAuthMethod(updated.VaultAuthMethod)
	if strings.TrimSpace(updated.VaultLogin) == "" {
		updated.VaultLogin = current.VaultLogin
	}
	if strings.TrimSpace(updated.KeePassPassword) != "" {
		if err := s.store.StoreSecret(securestorage.KeePassPasswordKey(), strings.TrimSpace(updated.KeePassPassword)); err != nil {
			return err
		}
	}
	if strings.TrimSpace(updated.VaultToken) != "" {
		if err := s.store.StoreSecret(securestorage.VaultTokenKey(), strings.TrimSpace(updated.VaultToken)); err != nil {
			return err
		}
	}
	if strings.TrimSpace(updated.VaultPassword) != "" {
		if err := s.store.StoreSecret(securestorage.VaultPasswordKey(), strings.TrimSpace(updated.VaultPassword)); err != nil {
			return err
		}
	}
	updated.VaultAddress = strings.TrimSpace(updated.VaultAddress)
	updated.VaultMountPoint = strings.Trim(strings.TrimSpace(updated.VaultMountPoint), "/")
	updated.VaultLogin = strings.TrimSpace(updated.VaultLogin)
	updated.KeePassDatabasePath = strings.TrimSpace(updated.KeePassDatabasePath)
	updated.KeePassPassword = ""
	updated.VaultToken = ""
	updated.VaultPassword = ""
	updated.HasKeePassPassword = s.store.SecretExists(securestorage.KeePassPasswordKey())
	updated.HasVaultToken = s.store.SecretExists(securestorage.VaultTokenKey())
	updated.HasVaultPassword = s.store.SecretExists(securestorage.VaultPasswordKey())
	updated.SSHForwardPorts = sanitizeForwardPorts(updated.SSHForwardPorts)
	updated.SSHForwardHostID = strings.TrimSpace(updated.SSHForwardHostID)
	updated.PortForwardRules = normalizeForwardRules(updated.PortForwardRules)
	return s.store.UpdateSettings(updated)
}

func normalizeVaultAuthMethod(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case vaultAuthMethodOIDC:
		return vaultAuthMethodOIDC
	case vaultAuthMethodOIDCSec:
		return vaultAuthMethodOIDCSec
	case vaultAuthMethodDomain:
		return vaultAuthMethodDomain
	default:
		return vaultAuthMethodToken
	}
}
func (s *Service) scrubSettingsForShell(appSettings settings.AppSettings) settings.AppSettings {
	scrubbed := appSettings
	scrubbed.VaultToken = ""
	scrubbed.VaultPassword = ""
	scrubbed.KeePassPassword = ""
	scrubbed.HasVaultToken = s.store.SecretExists(securestorage.VaultTokenKey())
	scrubbed.HasKeePassPassword = s.store.SecretExists(securestorage.KeePassPasswordKey())
	scrubbed.HasVaultPassword = s.store.SecretExists(securestorage.VaultPasswordKey())
	return scrubbed
}
