package app

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"eiksy/internal/domain/credentials"
	"eiksy/internal/securestorage"

	"github.com/tobischo/gokeepasslib/v3"
)

const maxCredentialResponseSize int64 = 1 << 20

func (s *Service) resolveCredentialReference(reference string) (string, error) {
	ref, err := parseCredentialReference(reference)
	if err != nil {
		return "", err
	}
	switch ref.ProviderID {
	case "vault":
		return s.loadVaultCredential(ref.Path, ref.Field)
	case "keepass":
		return s.loadKeePassCredential(ref.Path, ref.Field)
	default:
		return "", fmt.Errorf("unsupported credential provider %q", ref.ProviderID)
	}
}

func parseCredentialReference(reference string) (credentials.SecretReference, error) {
	reference = strings.TrimSpace(reference)
	provider, pathValue, ok := strings.Cut(reference, ":")
	provider = strings.ToLower(strings.TrimSpace(provider))
	pathValue = strings.TrimSpace(pathValue)
	if !ok || provider == "" || pathValue == "" {
		return credentials.SecretReference{}, fmt.Errorf("credential reference must use provider:path format")
	}

	field := "password"
	if pathPart, fieldPart, hasField := strings.Cut(pathValue, "#"); hasField {
		pathValue = strings.TrimSpace(pathPart)
		field = strings.TrimSpace(fieldPart)
		if field == "" {
			return credentials.SecretReference{}, fmt.Errorf("credential reference field is empty")
		}
	}
	if pathValue == "" {
		return credentials.SecretReference{}, fmt.Errorf("credential reference path is empty")
	}
	return credentials.SecretReference{ProviderID: provider, Path: pathValue, Field: field}, nil
}

func (s *Service) loadVaultCredential(secretPath, field string) (string, error) {
	cfg := s.store.Settings()
	if strings.TrimSpace(cfg.VaultAddress) == "" {
		return "", fmt.Errorf("vault address is not configured")
	}
	if strings.TrimSpace(cfg.VaultMountPoint) == "" {
		return "", fmt.Errorf("vault mountpoint is not configured")
	}

	baseURL, err := url.Parse(strings.TrimSpace(cfg.VaultAddress))
	if err != nil || baseURL.Host == "" || (baseURL.Scheme != "http" && baseURL.Scheme != "https") {
		return "", fmt.Errorf("vault address must be a valid http or https url")
	}
	token, err := s.resolveVaultAccessToken(baseURL, cfg)
	if err != nil {
		return "", err
	}
	if cfg.VaultAutoRenewToken {
		if err := s.renewVaultToken(baseURL, token); err != nil {
			s.EmitLog("warn", fmt.Sprintf("Vault token auto-renew failed: %v", err))
		}
	}

	endpoint := strings.TrimRight(baseURL.String(), "/") + "/v1/" + vaultPathJoin(cfg.VaultMountPoint, "data", secretPath)
	req, err := http.NewRequestWithContext(s.resolveContext(nil), http.MethodGet, endpoint, nil)
	if err != nil {
		return "", fmt.Errorf("build vault credential request: %w", err)
	}
	req.Header.Set("X-Vault-Token", token)

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("vault credential request failed: %w", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxCredentialResponseSize+1))
	if err != nil {
		return "", fmt.Errorf("read vault credential response: %w", err)
	}
	if int64(len(body)) > maxCredentialResponseSize {
		return "", fmt.Errorf("vault credential response exceeds %d bytes", maxCredentialResponseSize)
	}
	if resp.StatusCode == http.StatusNotFound {
		return "", fmt.Errorf("vault credential path %q was not found", secretPath)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("vault credential request returned %s", resp.Status)
	}

	var payload struct {
		Data map[string]json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return "", fmt.Errorf("parse vault credential response: %w", err)
	}
	raw, ok := payload.Data[field]
	if !ok {
		return "", fmt.Errorf("vault credential field %q was not found", field)
	}
	var value string
	if err := json.Unmarshal(raw, &value); err != nil {
		return "", fmt.Errorf("vault credential field %q is not a string", field)
	}
	if strings.TrimSpace(value) == "" {
		return "", fmt.Errorf("vault credential field %q is empty", field)
	}
	return value, nil
}

func (s *Service) loadKeePassCredential(secretPath, field string) (string, error) {
	cfg := s.store.Settings()
	dbPath := strings.TrimSpace(cfg.KeePassDatabasePath)
	if dbPath == "" {
		return "", fmt.Errorf("keepass database path is not configured")
	}
	if strings.HasPrefix(dbPath, "~/") {
		if homeDir, err := os.UserHomeDir(); err == nil && homeDir != "" {
			dbPath = filepath.Join(homeDir, strings.TrimPrefix(dbPath, "~/"))
		}
	}

	password, err := s.store.LoadSecret(securestorage.KeePassPasswordKey())
	if err != nil && !errors.Is(err, securestorage.ErrMasterPasswordRequired) {
		return "", err
	}
	password = strings.TrimSpace(password)
	if password == "" {
		return "", fmt.Errorf("keepass password is not configured")
	}

	file, err := os.Open(dbPath)
	if err != nil {
		return "", fmt.Errorf("open keepass database: %w", err)
	}
	defer file.Close()

	database := gokeepasslib.NewDatabase()
	database.Credentials = gokeepasslib.NewPasswordCredentials(password)
	if err := gokeepasslib.NewDecoder(file).Decode(database); err != nil {
		return "", fmt.Errorf("decode keepass database: %w", err)
	}
	if err := database.UnlockProtectedEntries(); err != nil {
		return "", fmt.Errorf("unlock keepass database entries: %w", err)
	}

	parts := strings.Split(strings.Trim(secretPath, "/"), "/")
	if len(parts) == 0 || strings.TrimSpace(parts[0]) == "" {
		return "", fmt.Errorf("keepass credential path is empty")
	}
	if len(database.Content.Root.Groups) == 0 {
		return "", fmt.Errorf("keepass database has no root group")
	}
	group := &database.Content.Root.Groups[0]
	for _, part := range parts[:len(parts)-1] {
		found := false
		for index := range group.Groups {
			if strings.TrimSpace(group.Groups[index].Name) == strings.TrimSpace(part) {
				group = &group.Groups[index]
				found = true
				break
			}
		}
		if !found {
			return "", fmt.Errorf("keepass credential path %q was not found", secretPath)
		}
	}

	leafName := strings.TrimSpace(parts[len(parts)-1])
	for index := range group.Entries {
		entry := &group.Entries[index]
		if strings.TrimSpace(entry.GetTitle()) != leafName {
			continue
		}
		value := entry.GetContent(field)
		if value == "" && strings.EqualFold(field, "password") {
			value = entry.GetPassword()
		}
		if strings.TrimSpace(value) == "" {
			return "", fmt.Errorf("keepass credential field %q is empty", field)
		}
		return value, nil
	}
	return "", fmt.Errorf("keepass credential path %q was not found", secretPath)
}
