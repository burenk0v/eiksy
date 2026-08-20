package disk

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestSettingsFilePersistsVaultAndAISecrets(t *testing.T) {
	baseDir := t.TempDir()

	store, err := NewStoreAt(baseDir)
	if err != nil {
		t.Fatalf("create disk store: %v", err)
	}

	cfg := store.Settings()
	cfg.VaultAddress = "https://vault.example.com"
	cfg.VaultMountPoint = "secret"
	cfg.VaultAutoRenewToken = true
	cfg.VaultToken = "vault-secret-token"
	if err := store.UpdateSettings(cfg); err != nil {
		t.Fatalf("update settings: %v", err)
	}

	aiState := store.AIState()
	if len(aiState.Providers) == 0 {
		t.Fatal("expected at least one ai provider")
	}
	aiState.Providers[0].Model = "gpt-5.6"
	aiState.Providers[0].Endpoint = "https://models.example.com/v1"
	aiState.Providers[0].Token = "ai-secret-token"
	aiState.Providers[0].Configured = true
	aiState.Providers[0].Status = "ready"
	store.UpdateAIState(aiState)

	settingsPath := filepath.Join(baseDir, "settings.json")
	raw, err := os.ReadFile(settingsPath)
	if err != nil {
		t.Fatalf("read settings file: %v", err)
	}

	var persisted map[string]any
	if err := json.Unmarshal(raw, &persisted); err != nil {
		t.Fatalf("decode settings file: %v", err)
	}
	if persisted["vaultToken"] != "vault-secret-token" {
		t.Fatalf("expected vault token in settings file, got %#v", persisted["vaultToken"])
	}
	if persisted["vaultAutoRenewToken"] != true {
		t.Fatalf("expected vaultAutoRenewToken in settings file, got %#v", persisted["vaultAutoRenewToken"])
	}

	aiStateRaw, ok := persisted["aiState"].(map[string]any)
	if !ok {
		t.Fatalf("expected aiState object in settings file, got %#v", persisted["aiState"])
	}
	providersRaw, ok := aiStateRaw["providers"].([]any)
	if !ok || len(providersRaw) == 0 {
		t.Fatalf("expected aiState.providers in settings file, got %#v", aiStateRaw["providers"])
	}
	firstProvider, ok := providersRaw[0].(map[string]any)
	if !ok {
		t.Fatalf("expected provider object, got %#v", providersRaw[0])
	}
	if firstProvider["token"] != "ai-secret-token" {
		t.Fatalf("expected ai token in settings file, got %#v", firstProvider["token"])
	}

	reloaded, err := NewStoreAt(baseDir)
	if err != nil {
		t.Fatalf("reload disk store: %v", err)
	}
	reloadedSettings := reloaded.Settings()
	if reloadedSettings.VaultToken != "vault-secret-token" {
		t.Fatalf("expected reloaded vault token, got %q", reloadedSettings.VaultToken)
	}
	if !reloadedSettings.VaultAutoRenewToken {
		t.Fatal("expected reloaded vault auto renew token setting")
	}

	reloadedAI := reloaded.AIState()
	if len(reloadedAI.Providers) == 0 {
		t.Fatal("expected reloaded ai provider")
	}
	if reloadedAI.Providers[0].Token != "ai-secret-token" {
		t.Fatalf("expected reloaded ai token, got %q", reloadedAI.Providers[0].Token)
	}
	if reloadedAI.Providers[0].Endpoint != "https://models.example.com/v1" {
		t.Fatalf("expected reloaded ai endpoint, got %q", reloadedAI.Providers[0].Endpoint)
	}
	if reloadedAI.Providers[0].Model != "gpt-5.6" {
		t.Fatalf("expected reloaded ai model, got %q", reloadedAI.Providers[0].Model)
	}
}
