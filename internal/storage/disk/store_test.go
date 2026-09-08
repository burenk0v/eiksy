package disk

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"opsy/internal/app"
	"opsy/internal/domain/sessions"
	"opsy/internal/domain/settings"
	"opsy/internal/securestorage"

	"github.com/zalando/go-keyring"
	_ "modernc.org/sqlite"
)

type memoryKeyring struct {
	items map[string]string
}

func newMemoryKeyring() *memoryKeyring {
	return &memoryKeyring{items: map[string]string{}}
}

func (m *memoryKeyring) key(service, user string) string {
	return service + ":" + user
}

func (m *memoryKeyring) Get(service, user string) (string, error) {
	value, ok := m.items[m.key(service, user)]
	if !ok {
		return "", keyring.ErrNotFound
	}
	return value, nil
}

func (m *memoryKeyring) Set(service, user, value string) error {
	m.items[m.key(service, user)] = value
	return nil
}

func (m *memoryKeyring) Delete(service, user string) error {
	delete(m.items, m.key(service, user))
	return nil
}

func TestSecretsMoveToEncryptedSQLiteStorage(t *testing.T) {
	baseDir := t.TempDir()
	keyring := newMemoryKeyring()

	store, err := NewStoreAtWithKeyring(baseDir, keyring)
	if err != nil {
		t.Fatalf("create disk store: %v", err)
	}
	service := app.NewService(store, nil, nil, nil)
	if status := service.GetSecureStorageStatus(); !status.Available || status.Configured || status.Unlocked {
		t.Fatalf("unexpected initial secure storage status: %+v", status)
	}
	if err := service.EnsureMasterPassword("master-password"); err != nil {
		t.Fatalf("ensure master password: %v", err)
	}

	cfg := store.Settings()
	cfg.VaultAddress = "https://vault.example.com"
	cfg.VaultMountPoint = "secret"
	cfg.VaultAutoRenewToken = true
	cfg.VaultProvider = "keepass"
	cfg.KeePassDatabasePath = "/tmp/keepass.kdbx"
	cfg.KeePassPassword = "keepass-secret"
	cfg.VaultToken = "vault-secret-token"
	if err := service.UpdateSettings(cfg); err != nil {
		t.Fatalf("update settings: %v", err)
	}
	if err := service.SaveCloudProvider("gpt-5.6", "https://models.example.com/v1", "ai-secret-token"); err != nil {
		t.Fatalf("save cloud provider: %v", err)
	}
	if err := service.CreateSessionProfile(sessions.Profile{
		ID:            "prod-ssh",
		Name:          "prod-ssh",
		ProtocolID:    "ssh",
		Host:          "prod.internal",
		Port:          22,
		Username:      "ops",
		Password:      sessions.EncryptedString("session-password"),
		KeyPassphrase: sessions.EncryptedString("ssh-key-passphrase"),
		Options: map[string]string{
			"auth_method":          "key",
			"ssh_private_key_path": "~/.ssh/id_ed25519",
		},
	}); err != nil {
		t.Fatalf("create session profile: %v", err)
	}

	settingsPath := filepath.Join(baseDir, "settings.json")
	rawSettings, err := os.ReadFile(settingsPath)
	if err != nil {
		t.Fatalf("read settings file: %v", err)
	}
	if strings.Contains(string(rawSettings), "vault-secret-token") || strings.Contains(string(rawSettings), "keepass-secret") || strings.Contains(string(rawSettings), "ai-secret-token") {
		t.Fatalf("settings.json must not contain plaintext secrets: %s", rawSettings)
	}
	var persistedSettings map[string]any
	if err := json.Unmarshal(rawSettings, &persistedSettings); err != nil {
		t.Fatalf("decode settings file: %v", err)
	}
	if _, ok := persistedSettings["vaultToken"]; ok {
		t.Fatalf("vault token must not be serialized to settings.json")
	}
	if _, ok := persistedSettings["keepassPassword"]; ok {
		t.Fatalf("keepass password must not be serialized to settings.json")
	}
	aiStateRaw := persistedSettings["aiState"].(map[string]any)
	firstProvider := aiStateRaw["providers"].([]any)[0].(map[string]any)
	if _, ok := firstProvider["token"]; ok {
		t.Fatalf("AI token must not be serialized to settings.json")
	}

	sessionsPath := filepath.Join(baseDir, "sessions.json")
	rawSessions, err := os.ReadFile(sessionsPath)
	if err != nil {
		t.Fatalf("read sessions file: %v", err)
	}
	if strings.Contains(string(rawSessions), "session-password") || strings.Contains(string(rawSessions), "ssh-key-passphrase") {
		t.Fatalf("sessions.json must not contain plaintext secrets: %s", rawSessions)
	}
	var persistedProfiles []map[string]any
	if err := json.Unmarshal(rawSessions, &persistedProfiles); err != nil {
		t.Fatalf("decode sessions file: %v", err)
	}
	if len(persistedProfiles) != 1 {
		t.Fatalf("expected one persisted session profile, got %d", len(persistedProfiles))
	}
	if _, ok := persistedProfiles[0]["password"]; ok {
		t.Fatalf("password must not be serialized to sessions.json")
	}
	if _, ok := persistedProfiles[0]["keyPassphrase"]; ok {
		t.Fatalf("key passphrase must not be serialized to sessions.json")
	}

	db, err := sql.Open("sqlite", filepath.Join(baseDir, "secrets.db"))
	if err != nil {
		t.Fatalf("open secrets db: %v", err)
	}
	defer db.Close()
	rows, err := db.Query(`SELECT name, value FROM secrets ORDER BY name`)
	if err != nil {
		t.Fatalf("query secrets db: %v", err)
	}
	defer rows.Close()
	var names []string
	for rows.Next() {
		var name, value string
		if err := rows.Scan(&name, &value); err != nil {
			t.Fatalf("scan secret row: %v", err)
		}
		names = append(names, name)
		if strings.Contains(value, "vault-secret-token") || strings.Contains(value, "keepass-secret") || strings.Contains(value, "ai-secret-token") || strings.Contains(value, "session-password") || strings.Contains(value, "ssh-key-passphrase") {
			t.Fatalf("secret row %q contains plaintext payload: %s", name, value)
		}
	}
	if len(names) != 4 {
		t.Fatalf("expected 4 encrypted secrets, got %d (%v)", len(names), names)
	}

	reloaded, err := NewStoreAtWithKeyring(baseDir, keyring)
	if err != nil {
		t.Fatalf("reload disk store: %v", err)
	}
	if status := reloaded.SecureStorageStatus(); !status.Available || !status.Configured || status.Unlocked {
		t.Fatalf("unexpected reloaded secure storage status: %+v", status)
	}
	if _, err := reloaded.LoadSecret(securestorage.VaultTokenKey()); err != securestorage.ErrMasterPasswordRequired {
		t.Fatalf("expected locked vault token to require master password, got %v", err)
	}
	if err := reloaded.EnsureMasterPassword("master-password"); err != nil {
		t.Fatalf("unlock reloaded secure storage: %v", err)
	}
	vaultToken, err := reloaded.LoadSecret(securestorage.VaultTokenKey())
	if err != nil {
		t.Fatalf("load reloaded vault token: %v", err)
	}
	if vaultToken != "vault-secret-token" {
		t.Fatalf("expected reloaded vault token, got %q", vaultToken)
	}
	keepassPassword, err := reloaded.LoadSecret(securestorage.KeePassPasswordKey())
	if err != nil {
		t.Fatalf("load reloaded keepass password: %v", err)
	}
	if keepassPassword != "keepass-secret" {
		t.Fatalf("expected reloaded keepass password, got %q", keepassPassword)
	}
	aiToken, err := reloaded.LoadSecret(securestorage.AIProviderTokenKey("openai-compatible-cloud"))
	if err != nil {
		t.Fatalf("load reloaded ai token: %v", err)
	}
	if aiToken != "ai-secret-token" {
		t.Fatalf("expected reloaded ai token, got %q", aiToken)
	}
	profilePassword, err := reloaded.LoadSecret(securestorage.SessionKeyPassphraseKey("prod-ssh"))
	if err != nil {
		t.Fatalf("load reloaded session passphrase: %v", err)
	}
	if profilePassword != "ssh-key-passphrase" {
		t.Fatalf("expected reloaded session passphrase, got %q", profilePassword)
	}
	profile := reloaded.SessionProfiles()[0]
	if !profile.HasKeyPassphrase {
		t.Fatal("expected persisted profile to advertise stored key passphrase")
	}
	if profile.HasPassword {
		t.Fatal("password secret should be removed when auth method is key")
	}
}

func TestUpdateSettingsStoresSecretFlagsWithoutPlaintext(t *testing.T) {
	baseDir := t.TempDir()
	store, err := NewStoreAtWithKeyring(baseDir, newMemoryKeyring())
	if err != nil {
		t.Fatalf("create disk store: %v", err)
	}
	if err := store.EnsureMasterPassword("master-password"); err != nil {
		t.Fatalf("ensure master password: %v", err)
	}
	service := app.NewService(store, nil, nil, nil)

	cfg := settings.AppSettings{
		VaultAddress:    "https://vault.example.com",
		VaultProvider:   "vault",
		VaultAuthMethod: "domain",
		VaultLogin:      "CORP\\ops",
		VaultToken:      "vault-token",
		VaultPassword:   "vault-password",
	}
	if err := service.UpdateSettings(cfg); err != nil {
		t.Fatalf("update settings: %v", err)
	}
	stored := service.GetShellState().Settings
	if !stored.HasVaultToken {
		t.Fatal("expected shell settings to advertise encrypted vault token")
	}
	if stored.VaultToken != "" {
		t.Fatalf("vault token must be scrubbed from shell state, got %q", stored.VaultToken)
	}
	if !stored.HasVaultPassword {
		t.Fatal("expected shell settings to advertise encrypted vault password")
	}
	if stored.VaultPassword != "" {
		t.Fatalf("vault password must be scrubbed from shell state, got %q", stored.VaultPassword)
	}
}

func TestLegacySecretsMigrateToSecureStorageAfterUnlock(t *testing.T) {
	baseDir := t.TempDir()
	keyring := newMemoryKeyring()

	legacySettings := map[string]any{
		"vaultAddress":        "https://vault.example.com",
		"vaultMountPoint":     "secret",
		"vaultProvider":       "vault",
		"vaultToken":          "legacy-vault-token",
		"keepassPassword":     "legacy-keepass-password",
		"keepassDatabasePath": "/tmp/legacy.kdbx",
		"aiState": map[string]any{
			"providers": []map[string]any{
				{
					"id":         "openai-compatible-cloud",
					"name":       "OpenAI-compatible Cloud",
					"class":      "openai_compatible",
					"model":      "sg-model",
					"endpoint":   "https://sourcegraph.example.com/.api/llm/openai/v1",
					"status":     "ready",
					"selected":   true,
					"token":      "legacy-ai-token",
					"configured": true,
				},
			},
			"contextPolicy": map[string]any{
				"sendTerminalSelection": true,
				"sendRecentOutput":      false,
				"requireConfirmation":   true,
			},
			"messages":      []map[string]any{},
			"chatSessionId": "chat-legacy",
		},
	}
	settingsBytes, err := json.MarshalIndent(legacySettings, "", "  ")
	if err != nil {
		t.Fatalf("marshal legacy settings: %v", err)
	}
	if err := os.WriteFile(filepath.Join(baseDir, "settings.json"), append(settingsBytes, '\n'), 0o600); err != nil {
		t.Fatalf("write legacy settings: %v", err)
	}

	legacyProfiles := []map[string]any{
		{
			"id":         "legacy-password",
			"name":       "legacy-password",
			"protocolId": "ssh",
			"host":       "legacy.internal",
			"port":       22,
			"username":   "ops",
			"password":   mustEncryptLegacySessionSecret(t, "legacy-session-password"),
			"options": map[string]any{
				"auth_method": "password",
			},
		},
		{
			"id":            "legacy-key",
			"name":          "legacy-key",
			"protocolId":    "ssh",
			"host":          "legacy.internal",
			"port":          22,
			"username":      "ops",
			"keyPassphrase": mustEncryptLegacySessionSecret(t, "legacy-key-passphrase"),
			"options": map[string]any{
				"auth_method":          "key",
				"ssh_private_key_path": "~/.ssh/id_ed25519",
			},
		},
	}
	profilesBytes, err := json.MarshalIndent(legacyProfiles, "", "  ")
	if err != nil {
		t.Fatalf("marshal legacy profiles: %v", err)
	}
	if err := os.WriteFile(filepath.Join(baseDir, "sessions.json"), append(profilesBytes, '\n'), 0o600); err != nil {
		t.Fatalf("write legacy sessions: %v", err)
	}

	store, err := NewStoreAtWithKeyring(baseDir, keyring)
	if err != nil {
		t.Fatalf("create disk store: %v", err)
	}
	service := app.NewService(store, nil, nil, nil)

	shell := service.GetShellState()
	if !shell.Settings.HasVaultToken || !shell.Settings.HasKeePassPassword {
		t.Fatalf("expected legacy settings secrets to be advertised, got %+v", shell.Settings)
	}
	if !shell.AI.Providers[0].HasToken {
		t.Fatal("expected legacy AI token to be advertised")
	}

	var passwordSeen, keyPassphraseSeen bool
	for _, profile := range shell.SessionProfiles {
		if profile.ID == "legacy-password" {
			passwordSeen = profile.HasPassword
		}
		if profile.ID == "legacy-key" {
			keyPassphraseSeen = profile.HasKeyPassphrase
		}
	}
	if !passwordSeen || !keyPassphraseSeen {
		t.Fatalf("expected legacy session secrets to be advertised")
	}

	if err := store.EnsureMasterPassword("master-password"); err != nil {
		t.Fatalf("ensure master password: %v", err)
	}

	if secret, err := store.LoadSecret(securestorage.VaultTokenKey()); err != nil || secret != "legacy-vault-token" {
		t.Fatalf("expected migrated vault token, got %q err=%v", secret, err)
	}
	if secret, err := store.LoadSecret(securestorage.KeePassPasswordKey()); err != nil || secret != "legacy-keepass-password" {
		t.Fatalf("expected migrated keepass password, got %q err=%v", secret, err)
	}
	if secret, err := store.LoadSecret(securestorage.AIProviderTokenKey("openai-compatible-cloud")); err != nil || secret != "legacy-ai-token" {
		t.Fatalf("expected migrated ai token, got %q err=%v", secret, err)
	}
	if secret, err := store.LoadSecret(securestorage.SessionPasswordKey("legacy-password")); err != nil || secret != "legacy-session-password" {
		t.Fatalf("expected migrated session password, got %q err=%v", secret, err)
	}
	if secret, err := store.LoadSecret(securestorage.SessionKeyPassphraseKey("legacy-key")); err != nil || secret != "legacy-key-passphrase" {
		t.Fatalf("expected migrated key passphrase, got %q err=%v", secret, err)
	}

	rawSettings, err := os.ReadFile(filepath.Join(baseDir, "settings.json"))
	if err != nil {
		t.Fatalf("read migrated settings: %v", err)
	}
	if strings.Contains(string(rawSettings), "legacy-vault-token") || strings.Contains(string(rawSettings), "legacy-keepass-password") || strings.Contains(string(rawSettings), "legacy-ai-token") {
		t.Fatalf("expected migrated settings.json to be scrubbed, got %s", rawSettings)
	}
	rawSessions, err := os.ReadFile(filepath.Join(baseDir, "sessions.json"))
	if err != nil {
		t.Fatalf("read migrated sessions: %v", err)
	}
	if strings.Contains(string(rawSessions), "legacy-session-password") || strings.Contains(string(rawSessions), "legacy-key-passphrase") {
		t.Fatalf("expected migrated sessions.json to be scrubbed, got %s", rawSessions)
	}
}

func mustEncryptLegacySessionSecret(t *testing.T, plaintext string) string {
	t.Helper()
	block, err := aes.NewCipher(legacySessionDerivedKey())
	if err != nil {
		t.Fatalf("create cipher: %v", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		t.Fatalf("create gcm: %v", err)
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		t.Fatalf("generate nonce: %v", err)
	}
	ciphertext := gcm.Seal(nonce, nonce, []byte(plaintext), nil)
	return base64.StdEncoding.EncodeToString(ciphertext)
}
