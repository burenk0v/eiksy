package sftpmanager

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestValidateLocalUploadPath(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "input.txt")
	if err := os.WriteFile(file, []byte("data"), 0o600); err != nil {
		t.Fatal(err)
	}

	got, err := validateLocalUploadPath(file)
	if err != nil {
		t.Fatalf("validateLocalUploadPath() error = %v", err)
	}
	if got != filepath.Clean(file) {
		t.Fatalf("validateLocalUploadPath() = %q, want %q", got, filepath.Clean(file))
	}
}

func TestValidateLocalUploadPathRejectsUnsafePaths(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "input.txt")
	if err := os.WriteFile(file, []byte("data"), 0o600); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name string
		path string
		want string
	}{
		{name: "relative", path: "input.txt", want: "absolute"},
		{name: "directory", path: dir, want: "regular file"},
		{name: "missing", path: filepath.Join(dir, "missing.txt"), want: "inspect local file"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := validateLocalUploadPath(tt.path)
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("validateLocalUploadPath(%q) error = %v, want error containing %q", tt.path, err, tt.want)
			}
		})
	}
}

func TestValidateLocalUploadPathRejectsSymlink(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "target.txt")
	link := filepath.Join(dir, "link.txt")
	if err := os.WriteFile(target, []byte("data"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, link); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}

	_, err := validateLocalUploadPath(link)
	if err == nil || !strings.Contains(err.Error(), "must not be a symlink") {
		t.Fatalf("validateLocalUploadPath() error = %v, want symlink rejection", err)
	}
}

func TestValidateLocalDownloadPathRequiresExistingParent(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "nested", "output.txt")

	_, err := validateLocalDownloadPath(path)
	if err == nil || !strings.Contains(err.Error(), "inspect local download directory") {
		t.Fatalf("validateLocalDownloadPath() error = %v, want missing-parent rejection", err)
	}
}

func TestValidateLocalDownloadPathRejectsDirectoryAndSymlink(t *testing.T) {
	dir := t.TempDir()

	_, err := validateLocalDownloadPath(dir)
	if err == nil || !strings.Contains(err.Error(), "is a directory") {
		t.Fatalf("validateLocalDownloadPath() error = %v, want directory rejection", err)
	}

	target := filepath.Join(dir, "target.txt")
	link := filepath.Join(dir, "link.txt")
	if err := os.WriteFile(target, []byte("data"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, link); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}

	_, err = validateLocalDownloadPath(link)
	if err == nil || !strings.Contains(err.Error(), "must not be a symlink") {
		t.Fatalf("validateLocalDownloadPath() error = %v, want symlink rejection", err)
	}
}

func TestValidateLocalDownloadPathAllowsNewFileInExistingDirectory(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "output.txt")

	got, err := validateLocalDownloadPath(path)
	if err != nil {
		t.Fatalf("validateLocalDownloadPath() error = %v", err)
	}
	if got != filepath.Clean(path) {
		t.Fatalf("validateLocalDownloadPath() = %q, want %q", got, filepath.Clean(path))
	}
}
