package securestorage

import (
	"path/filepath"
	"testing"

	"github.com/zalando/go-keyring"
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

func TestManagerReadsLegacyOpsyKeyringEntry(t *testing.T) {
	keyringStore := newMemoryKeyring()
	masterKey := make([]byte, masterKeySize)
	for i := range masterKey {
		masterKey[i] = byte(i + 1)
	}
	wrapped, err := wrapMasterKey("master-password", masterKey)
	if err != nil {
		t.Fatalf("wrap master key: %v", err)
	}
	if err := keyringStore.Set(legacyServiceName, defaultKeyringUser, wrapped); err != nil {
		t.Fatalf("seed legacy keyring entry: %v", err)
	}

	manager, err := NewWithKeyring(filepath.Join(t.TempDir(), "secrets.db"), keyringStore)
	if err != nil {
		t.Fatalf("create manager: %v", err)
	}
	defer manager.Close()

	status := manager.Status()
	if !status.Available || !status.Configured || status.Unlocked {
		t.Fatalf("unexpected status from legacy keyring entry: %+v", status)
	}
	if err := manager.EnsureMasterPassword("master-password"); err != nil {
		t.Fatalf("unlock with legacy keyring entry: %v", err)
	}
	if _, err := keyringStore.Get(defaultServiceName, defaultKeyringUser); err != nil {
		t.Fatalf("expected migrated eiksy keyring entry: %v", err)
	}
}
