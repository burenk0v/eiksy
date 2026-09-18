package app

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"eiksy/internal/domain/settings"
	"eiksy/internal/securestorage"
)

func (s *Service) renewVaultToken(baseURL *url.URL, token string) error {
	endpoint := strings.TrimRight(baseURL.String(), "/") + "/v1/auth/token/renew-self"
	req, err := http.NewRequestWithContext(s.resolveContext(nil), http.MethodPost, endpoint, bytes.NewReader([]byte(`{}`)))
	if err != nil {
		return fmt.Errorf("build vault token renewal request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Vault-Token", token)

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("vault token renewal failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxVaultResponseSize+1))
	if err != nil {
		return fmt.Errorf("read vault token renewal response: %w", err)
	}
	if int64(len(body)) > maxVaultResponseSize {
		return fmt.Errorf("vault token renewal response exceeds %d bytes", maxVaultResponseSize)
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("vault token renewal returned %s: %s", resp.Status, strings.TrimSpace(string(body)))
	}
	return nil
}

func (s *Service) resolveVaultAccessToken(baseURL *url.URL, cfg settings.AppSettings) (string, error) {
	authMethod := normalizeVaultAuthMethod(cfg.VaultAuthMethod)
	if authMethod == vaultAuthMethodToken {
		token, err := s.store.LoadSecret(securestorage.VaultTokenKey())
		if err != nil && !errors.Is(err, securestorage.ErrMasterPasswordRequired) {
			return "", err
		}
		token = strings.TrimSpace(token)
		if token == "" {
			return "", fmt.Errorf("vault token is not configured")
		}
		return token, nil
	}

	login := strings.TrimSpace(cfg.VaultLogin)
	if login == "" {
		return "", fmt.Errorf("vault login is not configured")
	}
	password, err := s.store.LoadSecret(securestorage.VaultPasswordKey())
	if err != nil && !errors.Is(err, securestorage.ErrMasterPasswordRequired) {
		return "", err
	}
	password = strings.TrimSpace(password)
	if password == "" {
		return "", fmt.Errorf("vault password is not configured")
	}
	return s.loginVault(baseURL, authMethod, login, password)
}

func (s *Service) loginVault(baseURL *url.URL, authMethod, login, password string) (string, error) {
	endpoint := strings.TrimRight(baseURL.String(), "/") + "/v1/auth/" + authMethod + "/login/" + url.PathEscape(login)
	payload, err := json.Marshal(map[string]string{"password": password})
	if err != nil {
		return "", fmt.Errorf("build vault login payload: %w", err)
	}

	req, err := http.NewRequestWithContext(s.resolveContext(nil), http.MethodPost, endpoint, bytes.NewReader(payload))
	if err != nil {
		return "", fmt.Errorf("build vault login request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("vault login failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxVaultResponseSize+1))
	if err != nil {
		return "", fmt.Errorf("read vault login response: %w", err)
	}
	if int64(len(body)) > maxVaultResponseSize {
		return "", fmt.Errorf("vault login response exceeds %d bytes", maxVaultResponseSize)
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("vault login returned %s: %s", resp.Status, strings.TrimSpace(string(body)))
	}

	var parsed struct {
		Auth struct {
			ClientToken string `json:"client_token"`
		} `json:"auth"`
	}
	if err := json.Unmarshal(body, &parsed); err != nil {
		return "", fmt.Errorf("parse vault login response: %w", err)
	}
	token := strings.TrimSpace(parsed.Auth.ClientToken)
	if token == "" {
		return "", fmt.Errorf("vault login response missing client token")
	}
	return token, nil
}

func vaultPathJoin(parts ...string) string {
	filtered := make([]string, 0, len(parts))
	for _, part := range parts {
		p := strings.Trim(strings.TrimSpace(part), "/")
		if p == "" {
			continue
		}
		for _, token := range strings.Split(p, "/") {
			escaped := url.PathEscape(strings.TrimSpace(token))
			if escaped != "" {
				filtered = append(filtered, escaped)
			}
		}
	}
	return strings.Join(filtered, "/")
}
