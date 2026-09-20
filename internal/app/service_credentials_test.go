package app

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"eiksy/internal/securestorage"
	"github.com/tobischo/gokeepasslib/v3"
	"eiksy/internal/storage/memory"
)

func TestParseCredentialReference(t *testing.T) {
	ref, err := parseCredentialReference("vault:team/database#username")
	if err != nil {
		t.Fatalf("parse reference: %v", err)
	}
	if ref.ProviderID != "vault" || ref.Path != "team/database" || ref.Field != "username" {
		t.Fatalf("unexpected reference: %#v", ref)
	}

	ref, err = parseCredentialReference("keepass:Prod/SSH")
	if err != nil {
		t.Fatalf("parse default field reference: %v", err)
	}
	if ref.Field != "password" {
		t.Fatalf("expected default password field, got %q", ref.Field)
	}
}

func TestParseCredentialReferenceRejectsInvalidReference(t *testing.T) {
	for _, reference := range []string{
		"", "vault", ":secret", "vault:", "vault:secret#",
		"unknown:secret", "vault:../secret", "vault:team/../secret",
		"vault:team/secret\x00name", "vault:.", "vault:./secret",
	} {
		if _, err := parseCredentialReference(reference); err == nil {
			t.Fatalf("expected invalid reference %q to fail", reference)
		}
	}
}

func TestParseCredentialReferenceNormalizesPath(t *testing.T) {
	ref, err := parseCredentialReference("VAULT:/team/prod/password/")
	if err != nil {
		t.Fatalf("parse reference: %v", err)
	}
	if ref.ProviderID != "vault" || ref.Path != "team/prod/password" || ref.Field != "password" {
		t.Fatalf("unexpected normalized reference: %#v", ref)
	}
}

func TestLoadKeePassCredentialResolvesNestedEntry(t *testing.T) {
	db := gokeepasslib.NewDatabase()
	db.Credentials = gokeepasslib.NewPasswordCredentials("master-password")

	root := gokeepasslib.NewGroup()
	root.Name = "Root"
	production := gokeepasslib.NewGroup()
	production.Name = "Production"
	entry := gokeepasslib.NewEntry()
	entry.Values = []gokeepasslib.ValueData{
		{Key: "Title", Value: gokeepasslib.V{Content: "SSH"}},
		{Key: "UserName", Value: gokeepasslib.V{Content: "alice"}},
		{Key: "Password", Value: gokeepasslib.V{Content: "keepass-secret"}},
	}
	production.Entries = []gokeepasslib.Entry{entry}
	root.Groups = []gokeepasslib.Group{production}
	db.Content.Root.Groups = []gokeepasslib.Group{root}

	dir := t.TempDir()
	dbPath := filepath.Join(dir, "credentials.kdbx")
	file, err := os.Create(dbPath)
	if err != nil {
		t.Fatalf("create KeePass database: %v", err)
	}
	if err := gokeepasslib.NewEncoder(file).Encode(db); err != nil {
		_ = file.Close()
		t.Fatalf("encode KeePass database: %v", err)
	}
	if err := file.Close(); err != nil {
		t.Fatalf("close KeePass database: %v", err)
	}

	store := memory.NewStore()
	cfg := store.Settings()
	cfg.KeePassDatabasePath = dbPath
	if err := store.UpdateSettings(cfg); err != nil {
		t.Fatalf("update settings: %v", err)
	}
	if err := store.StoreSecret(securestorage.KeePassPasswordKey(), "master-password"); err != nil {
		t.Fatalf("store KeePass password: %v", err)
	}

	service := NewService(store, nil, nil)
	value, err := service.resolveCredentialReference("keepass:Production/SSH")
	if err != nil {
		t.Fatalf("resolve KeePass credential: %v", err)
	}
	if value != "keepass-secret" {
		t.Fatalf("unexpected KeePass credential: %q", value)
	}
}

func TestLoadVaultCredentialReadsOnlyRequestedField(t *testing.T) {
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		if r.Method != http.MethodGet || r.URL.Path != "/v1/secret/data/team/database" {
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		if r.Header.Get("X-Vault-Token") != "test-token" {
			t.Fatalf("unexpected Vault token")
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":{"username":"alice","password":"super-secret"}}`))
	}))
	defer server.Close()

	store := memory.NewStore()
	cfg := store.Settings()
	cfg.VaultAddress = server.URL
	cfg.VaultMountPoint = "secret"
	if err := store.UpdateSettings(cfg); err != nil {
		t.Fatalf("update settings: %v", err)
	}
	if err := store.StoreSecret(securestorage.VaultTokenKey(), "test-token"); err != nil {
		t.Fatalf("store token: %v", err)
	}

	service := NewService(store, nil, nil)
	service.httpClient = server.Client()

	value, err := service.resolveCredentialReference("vault:team/database#password")
	if err != nil {
		t.Fatalf("resolve Vault credential: %v", err)
	}
	if value != "super-secret" {
		t.Fatalf("unexpected credential value: %q", value)
	}
	if requests != 1 {
		t.Fatalf("expected one Vault request, got %d", requests)
	}
}

func TestLoadVaultCredentialDoesNotExposeMissingFieldValue(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":{"username":"alice"}}`))
	}))
	defer server.Close()

	store := memory.NewStore()
	cfg := store.Settings()
	cfg.VaultAddress = server.URL
	cfg.VaultMountPoint = "secret"
	if err := store.UpdateSettings(cfg); err != nil {
		t.Fatalf("update settings: %v", err)
	}
	if err := store.StoreSecret(securestorage.VaultTokenKey(), "test-token"); err != nil {
		t.Fatalf("store token: %v", err)
	}

	service := NewService(store, nil, nil)
	service.httpClient = server.Client()
	if _, err := service.resolveCredentialReference("vault:team/database#password"); err == nil {
		t.Fatal("expected missing Vault field error")
	}
}
