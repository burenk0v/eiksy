package main

import (
	"path/filepath"
	"runtime"
	"testing"
)

func TestWebviewUserDataPath(t *testing.T) {
	path := webviewUserDataPath()

	if runtime.GOOS != "windows" {
		if path != "" {
			t.Fatalf("expected empty WebView2 data path on %s, got %q", runtime.GOOS, path)
		}
		return
	}

	if filepath.Base(path) != webviewUserDataDirName {
		t.Fatalf("expected WebView2 data directory %q, got %q", webviewUserDataDirName, path)
	}
}
