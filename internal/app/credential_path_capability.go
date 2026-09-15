package app

import (
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"eiksy/internal/domain/settings"
	"eiksy/internal/securestorage"

	"github.com/tobischo/gokeepasslib/v3"
)

// credentialPathExistsForProvider checks only the explicitly requested leaf
// path. It deliberately does not return or enumerate sibling secrets.
func (s *Service) credentialPathExistsForProvider(provider, normalizedPath string) (bool, error) {
	switch provider {
	case "vault":
		return s.vaultCredentialPathExists(normalizedPath)
	case "keepass":
		return s.keepassCredentialPathExists(normalizedPath)
	default:
		return false, fmt.Errorf("unsupported credential provider %q", provider)
	}
}

func (s *Service) vaultCredentialPathExists(secretPath string) (bool, error) {
	cfg := s.store.Settings()
	if strings.TrimSpace(cfg.VaultAddress) == "" {
		return false, fmt.Errorf("vault address is not configured")
	}
	if strings.TrimSpace(cfg.VaultMountPoint) == "" {
		return false, fmt.Errorf("vault mountpoint is not configured")
	}

	parsed, err := url.Parse(strings.TrimSpace(cfg.VaultAddress))
	if err != nil || parsed.Host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return false, fmt.Errorf("vault address must be a valid http or https url")
	}

	token, err := s.resolveVaultAccessToken(parsed, cfg)
	if err != nil {
		return false, err
	}
	if cfg.VaultAutoRenewToken {
		if err := s.renewVaultToken(parsed, token); err != nil {
			s.EmitLog("warn", fmt.Sprintf("Vault token auto-renew failed: %v", err))
		}
	}

	metadataRoute := vaultPathJoin(cfg.VaultMountPoint, "metadata", secretPath)
	endpoint := strings.TrimRight(parsed.String(), "/") + "/v1/" + metadataRoute
	req, err := http.NewRequestWithContext(s.resolveContext(nil), http.MethodGet, endpoint, nil)
	if err != nil {
		return false, fmt.Errorf("build vault credential-path request: %w", err)
	}
	req.Header.Set("X-Vault-Token", token)

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return false, fmt.Errorf("vault credential-path request failed: %w", err)
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusOK:
		return true, nil
	case http.StatusNotFound:
		return false, nil
	default:
		return false, fmt.Errorf("vault credential-path request returned %s", resp.Status)
	}
}

func (s *Service) keepassCredentialPathExists(secretPath string) (bool, error) {
	cfg := s.store.Settings()
	dbPath := strings.TrimSpace(cfg.KeePassDatabasePath)
	if dbPath == "" {
		return false, fmt.Errorf("keepass database path is not configured")
	}
	if strings.HasPrefix(dbPath, "~/") {
		if homeDir, err := os.UserHomeDir(); err == nil && homeDir != "" {
			dbPath = filepath.Join(homeDir, strings.TrimPrefix(dbPath, "~/"))
		}
	}

	password, err := s.store.LoadSecret(securestorage.KeePassPasswordKey())
	if err != nil && !errors.Is(err, securestorage.ErrMasterPasswordRequired) {
		return false, err
	}
	password = strings.TrimSpace(password)
	if password == "" {
		return false, fmt.Errorf("keepass password is not configured")
	}

	file, err := os.Open(dbPath)
	if err != nil {
		return false, fmt.Errorf("open keepass database: %w", err)
	}
	defer file.Close()

	database := gokeepasslib.NewDatabase()
	database.Credentials = gokeepasslib.NewPasswordCredentials(password)
	if err := gokeepasslib.NewDecoder(file).Decode(database); err != nil {
		return false, fmt.Errorf("decode keepass database: %w", err)
	}
	if err := database.UnlockProtectedEntries(); err != nil {
		return false, fmt.Errorf("unlock keepass database entries: %w", err)
	}
	if len(database.Content.Root.Groups) == 0 {
		return false, fmt.Errorf("keepass database does not contain root groups")
	}

	parts := strings.Split(secretPath, "/")
	leafName := parts[len(parts)-1]
	group := &database.Content.Root.Groups[0]
	for _, part := range parts[:len(parts)-1] {
		found := false
		for index := range group.Groups {
			if strings.TrimSpace(group.Groups[index].Name) == part {
				group = &group.Groups[index]
				found = true
				break
			}
		}
		if !found {
			return false, nil
		}
	}

	for _, entry := range group.Entries {
		if strings.TrimSpace(entry.GetTitle()) == leafName {
			return true, nil
		}
	}
	return false, nil
}

var _ settings.AppSettings
