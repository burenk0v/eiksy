package sessions

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
)

const masterKey = "zg4ewf1u"

// EncryptedString stores the plaintext password in memory and transparently
// encrypts it with AES-256-GCM when serialising to JSON (e.g. sessions.json).
// The master key is compiled into the binary.
// ts_type string
type EncryptedString string

func derivedKey() []byte {
	sum := sha256.Sum256([]byte(masterKey))
	return sum[:]
}

func encryptPassword(plaintext string) (string, error) {
	block, err := aes.NewCipher(derivedKey())
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err = io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}
	ciphertext := gcm.Seal(nonce, nonce, []byte(plaintext), nil)
	return base64.StdEncoding.EncodeToString(ciphertext), nil
}

func decryptPassword(encoded string) (string, error) {
	data, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return "", err
	}
	block, err := aes.NewCipher(derivedKey())
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	nonceSize := gcm.NonceSize()
	if len(data) < nonceSize {
		return "", fmt.Errorf("ciphertext too short")
	}
	plaintext, err := gcm.Open(nil, data[:nonceSize], data[nonceSize:], nil)
	if err != nil {
		return "", err
	}
	return string(plaintext), nil
}

// MarshalJSON encrypts the plaintext password before writing to disk.
func (e EncryptedString) MarshalJSON() ([]byte, error) {
	if string(e) == "" {
		return json.Marshal("")
	}
	encrypted, err := encryptPassword(string(e))
	if err != nil {
		return nil, err
	}
	return json.Marshal(encrypted)
}

// UnmarshalJSON decrypts the stored value when loading from disk.
// If decryption fails (e.g. legacy base64-only data from an older version of the
// application), the password is silently reset to empty so that startup succeeds;
// the user will need to re-enter the password for affected sessions when reconnecting.
func (e *EncryptedString) UnmarshalJSON(data []byte) error {
	var encoded string
	if err := json.Unmarshal(data, &encoded); err != nil {
		return err
	}
	if encoded == "" {
		*e = ""
		return nil
	}
	plaintext, err := decryptPassword(encoded)
	if err != nil {
		// Legacy or corrupt data: reset to empty rather than failing startup.
		// The user will be prompted to re-enter the password when connecting.
		*e = ""
		return nil
	}
	*e = EncryptedString(plaintext)
	return nil
}

type Profile struct {
	ID             string            `json:"id"`
	Name           string            `json:"name"`
	Group          string            `json:"group"`
	Tags           []string          `json:"tags"`
	Favorite       bool              `json:"favorite"`
	ProtocolID     string            `json:"protocolId"`
	Host           string            `json:"host"`
	Port           int               `json:"port"`
	Username       string            `json:"username"`
	Password       EncryptedString   `json:"password,omitempty"`
	SecretRef      string            `json:"secretRef,omitempty"`
	Options        map[string]string `json:"options,omitempty"`
	LastLaunchedAt string            `json:"lastLaunchedAt,omitempty"`
}

// ProfileInput is used as the parameter type for CreateSessionProfile in the
// Wails API. Unlike Profile, it exposes the Password field in JSON so that
// Wails generates the corresponding TypeScript property. The password is never
// persisted to disk; it is only used in-flight during the IPC call.
type ProfileInput struct {
	ID             string            `json:"id"`
	Name           string            `json:"name"`
	Group          string            `json:"group"`
	Tags           []string          `json:"tags"`
	Favorite       bool              `json:"favorite"`
	ProtocolID     string            `json:"protocolId"`
	Host           string            `json:"host"`
	Port           int               `json:"port"`
	Username       string            `json:"username"`
	Password       string            `json:"password,omitempty"`
	SecretRef      string            `json:"secretRef,omitempty"`
	Options        map[string]string `json:"options,omitempty"`
	LastLaunchedAt string            `json:"lastLaunchedAt,omitempty"`
}

// ToProfile converts a ProfileInput to a Profile, copying all fields including
// the password (which is not persisted when the Profile is later saved).
func (p ProfileInput) ToProfile() Profile {
	return Profile{
		ID:             p.ID,
		Name:           p.Name,
		Group:          p.Group,
		Tags:           p.Tags,
		Favorite:       p.Favorite,
		ProtocolID:     p.ProtocolID,
		Host:           p.Host,
		Port:           p.Port,
		Username:       p.Username,
		Password:       EncryptedString(p.Password),
		SecretRef:      p.SecretRef,
		Options:        p.Options,
		LastLaunchedAt: p.LastLaunchedAt,
	}
}

type HistoryEntry struct {
	ProfileID   string `json:"profileId"`
	ProfileName string `json:"profileName"`
	LaunchedAt  string `json:"launchedAt"`
}
