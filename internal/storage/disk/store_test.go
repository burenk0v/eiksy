package disk

import (
	"database/sql"
	"encoding/json"
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
