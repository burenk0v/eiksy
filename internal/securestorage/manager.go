package securestorage

import (
	"crypto/rand"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/zalando/go-keyring"
	"golang.org/x/crypto/argon2"
	"golang.org/x/crypto/chacha20poly1305"
	_ "modernc.org/sqlite"
)

const (
	defaultServiceName = "opsy"
	defaultKeyringUser = "master-key"
	masterKeySize      = 32
	saltSize           = 16
	argonTime          = 3
	argonMemoryKiB     = 64 * 1024
	argonThreads       = 4
)

var (
	ErrMasterPasswordRequired = errors.New("master password required")
	ErrInvalidMasterPassword  = errors.New("invalid master password")
)

type Status struct {
	Available  bool `json:"available"`
	Configured bool `json:"configured"`
	Unlocked   bool `json:"unlocked"`
}

type Keyring interface {
	Get(service, user string) (string, error)
	Set(service, user, value string) error
	Delete(service, user string) error
}

type Manager struct {
	mu          sync.RWMutex
	db          *sql.DB
	keyring     Keyring
	serviceName string
	keyringUser string
	masterKey   []byte
}

type wrappedMasterKeyRecord struct {
	Version    int    `json:"version"`
	Salt       string `json:"salt"`
	Nonce      string `json:"nonce"`
	Ciphertext string `json:"ciphertext"`
}

type encryptedSecretRecord struct {
	Version    int    `json:"version"`
	Nonce      string `json:"nonce"`
	Ciphertext string `json:"ciphertext"`
}

func New(dbPath string) (*Manager, error) {
	return NewWithKeyring(dbPath, systemKeyring{})
}

func NewWithKeyring(dbPath string, keyring Keyring) (*Manager, error) {
	if err := os.MkdirAll(filepath.Dir(dbPath), 0o755); err != nil {
		return nil, fmt.Errorf("create secure storage directory: %w", err)
	}
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("open secrets database: %w", err)
	}
	manager := &Manager{
		db:          db,
		keyring:     keyring,
		serviceName: defaultServiceName,
		keyringUser: defaultKeyringUser,
	}
	if err := manager.initSchema(); err != nil {
		_ = db.Close()
		return nil, err
	}
	return manager, nil
}

func (m *Manager) Close() error {
	if m == nil || m.db == nil {
		return nil
	}
	return m.db.Close()
}

func (m *Manager) Status() Status {
	status := Status{Available: true}
	record, err := m.keyring.Get(m.serviceName, m.keyringUser)
	switch {
	case err == nil:
		status.Configured = record != ""
	case errors.Is(err, keyring.ErrNotFound):
		status.Configured = false
	default:
		status.Available = false
	}
	m.mu.RLock()
	status.Unlocked = len(m.masterKey) == masterKeySize
	m.mu.RUnlock()
	return status
}

func (m *Manager) EnsureMasterPassword(password string) error {
	if password == "" {
		return fmt.Errorf("master password is required")
	}
	record, err := m.keyring.Get(m.serviceName, m.keyringUser)
	switch {
	case err == nil:
		masterKey, err := unwrapMasterKey(password, record)
		if err != nil {
			return err
		}
		m.setMasterKey(masterKey)
		return nil
	case errors.Is(err, keyring.ErrNotFound):
		masterKey := make([]byte, masterKeySize)
		if _, err := rand.Read(masterKey); err != nil {
			return fmt.Errorf("generate master key: %w", err)
		}
		wrapped, err := wrapMasterKey(password, masterKey)
		if err != nil {
			return err
		}
		if err := m.keyring.Set(m.serviceName, m.keyringUser, wrapped); err != nil {
			return fmt.Errorf("store master key in keychain: %w", err)
		}
		m.setMasterKey(masterKey)
		return nil
	default:
		return fmt.Errorf("access os keychain: %w", err)
	}
}

func (m *Manager) SecretExists(key string) bool {
	if stringsTrimSpace(key) == "" || m == nil || m.db == nil {
		return false
	}
	var exists int
	if err := m.db.QueryRow(`SELECT 1 FROM secrets WHERE name = ? LIMIT 1`, key).Scan(&exists); err != nil {
		return false
	}
	return exists == 1
}

func (m *Manager) StoreSecret(key, plaintext string) error {
	if stringsTrimSpace(key) == "" {
		return fmt.Errorf("secret key is required")
	}
	masterKey, err := m.requireUnlockedMasterKey()
	if err != nil {
		return err
	}
	sealed, err := encryptSecret(masterKey, plaintext)
	if err != nil {
		return err
	}
	_, err = m.db.Exec(`
		INSERT INTO secrets(name, value) VALUES(?, ?)
		ON CONFLICT(name) DO UPDATE SET value = excluded.value
	`, key, sealed)
	if err != nil {
		return fmt.Errorf("store secret %q: %w", key, err)
	}
	return nil
}

func (m *Manager) LoadSecret(key string) (string, error) {
	if stringsTrimSpace(key) == "" {
		return "", nil
	}
	var payload string
	if err := m.db.QueryRow(`SELECT value FROM secrets WHERE name = ?`, key).Scan(&payload); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", nil
		}
		return "", fmt.Errorf("load secret %q: %w", key, err)
	}
	masterKey, err := m.requireUnlockedMasterKey()
	if err != nil {
		return "", err
	}
	return decryptSecret(masterKey, payload)
}

func (m *Manager) DeleteSecret(key string) error {
	if stringsTrimSpace(key) == "" || m == nil || m.db == nil {
		return nil
	}
	if _, err := m.db.Exec(`DELETE FROM secrets WHERE name = ?`, key); err != nil {
		return fmt.Errorf("delete secret %q: %w", key, err)
	}
	return nil
}

func (m *Manager) initSchema() error {
	if _, err := m.db.Exec(`
		CREATE TABLE IF NOT EXISTS secrets (
			name TEXT PRIMARY KEY,
			value TEXT NOT NULL
		)
	`); err != nil {
		return fmt.Errorf("initialise secrets database: %w", err)
	}
	return nil
}

func (m *Manager) requireUnlockedMasterKey() ([]byte, error) {
	status := m.Status()
	if !status.Available || !status.Configured || !status.Unlocked {
		return nil, ErrMasterPasswordRequired
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	masterKey := append([]byte(nil), m.masterKey...)
	if len(masterKey) != masterKeySize {
		return nil, ErrMasterPasswordRequired
	}
	return masterKey, nil
}

func (m *Manager) setMasterKey(masterKey []byte) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.masterKey = append([]byte(nil), masterKey...)
}

func wrapMasterKey(password string, masterKey []byte) (string, error) {
	salt := make([]byte, saltSize)
	if _, err := rand.Read(salt); err != nil {
		return "", fmt.Errorf("generate master-key salt: %w", err)
	}
	derived := deriveKey(password, salt)
	sealed, nonce, err := seal(derived, masterKey)
	if err != nil {
		return "", err
	}
	record, err := json.Marshal(wrappedMasterKeyRecord{
		Version:    1,
		Salt:       base64.StdEncoding.EncodeToString(salt),
		Nonce:      base64.StdEncoding.EncodeToString(nonce),
		Ciphertext: base64.StdEncoding.EncodeToString(sealed),
	})
	if err != nil {
		return "", fmt.Errorf("encode wrapped master key: %w", err)
	}
	return string(record), nil
}

func unwrapMasterKey(password string, payload string) ([]byte, error) {
	var record wrappedMasterKeyRecord
	if err := json.Unmarshal([]byte(payload), &record); err != nil {
		return nil, fmt.Errorf("decode wrapped master key: %w", err)
	}
	salt, err := base64.StdEncoding.DecodeString(record.Salt)
	if err != nil {
		return nil, fmt.Errorf("decode wrapped master key salt: %w", err)
	}
	nonce, err := base64.StdEncoding.DecodeString(record.Nonce)
	if err != nil {
		return nil, fmt.Errorf("decode wrapped master key nonce: %w", err)
	}
	ciphertext, err := base64.StdEncoding.DecodeString(record.Ciphertext)
	if err != nil {
		return nil, fmt.Errorf("decode wrapped master key ciphertext: %w", err)
	}
	derived := deriveKey(password, salt)
	masterKey, err := open(derived, nonce, ciphertext)
	if err != nil {
		return nil, ErrInvalidMasterPassword
	}
	return masterKey, nil
}

func encryptSecret(masterKey []byte, plaintext string) (string, error) {
	sealed, nonce, err := seal(masterKey, []byte(plaintext))
	if err != nil {
		return "", err
	}
	record, err := json.Marshal(encryptedSecretRecord{
		Version:    1,
		Nonce:      base64.StdEncoding.EncodeToString(nonce),
		Ciphertext: base64.StdEncoding.EncodeToString(sealed),
	})
	if err != nil {
		return "", fmt.Errorf("encode encrypted secret: %w", err)
	}
	return string(record), nil
}

func decryptSecret(masterKey []byte, payload string) (string, error) {
	var record encryptedSecretRecord
	if err := json.Unmarshal([]byte(payload), &record); err != nil {
		return "", fmt.Errorf("decode encrypted secret: %w", err)
	}
	nonce, err := base64.StdEncoding.DecodeString(record.Nonce)
	if err != nil {
		return "", fmt.Errorf("decode encrypted secret nonce: %w", err)
	}
	ciphertext, err := base64.StdEncoding.DecodeString(record.Ciphertext)
	if err != nil {
		return "", fmt.Errorf("decode encrypted secret ciphertext: %w", err)
	}
	plaintext, err := open(masterKey, nonce, ciphertext)
	if err != nil {
		return "", fmt.Errorf("decrypt secret: %w", err)
	}
	return string(plaintext), nil
}

func deriveKey(password string, salt []byte) []byte {
	return argon2.IDKey([]byte(password), salt, argonTime, argonMemoryKiB, argonThreads, chacha20poly1305.KeySize)
}

func seal(key []byte, plaintext []byte) ([]byte, []byte, error) {
	aead, err := chacha20poly1305.NewX(key)
	if err != nil {
		return nil, nil, fmt.Errorf("create xchacha20-poly1305 cipher: %w", err)
	}
	nonce := make([]byte, chacha20poly1305.NonceSizeX)
	if _, err := rand.Read(nonce); err != nil {
		return nil, nil, fmt.Errorf("generate xchacha20 nonce: %w", err)
	}
	return aead.Seal(nil, nonce, plaintext, nil), nonce, nil
}

func open(key []byte, nonce []byte, ciphertext []byte) ([]byte, error) {
	aead, err := chacha20poly1305.NewX(key)
	if err != nil {
		return nil, fmt.Errorf("create xchacha20-poly1305 cipher: %w", err)
	}
	plaintext, err := aead.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, err
	}
	return plaintext, nil
}

func stringsTrimSpace(value string) string {
	start := 0
	end := len(value)
	for start < end {
		switch value[start] {
		case ' ', '\n', '\r', '\t':
			start++
		default:
			goto trimEnd
		}
	}
trimEnd:
	for end > start {
		switch value[end-1] {
		case ' ', '\n', '\r', '\t':
			end--
		default:
			return value[start:end]
		}
	}
	return value[start:end]
}

type systemKeyring struct{}

func (systemKeyring) Get(service, user string) (string, error) {
	return keyring.Get(service, user)
}

func (systemKeyring) Set(service, user, value string) error {
	return keyring.Set(service, user, value)
}

func (systemKeyring) Delete(service, user string) error {
	return keyring.Delete(service, user)
}
