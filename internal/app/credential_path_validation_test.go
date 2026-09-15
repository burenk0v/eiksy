package app

import (
	"strings"
	"testing"
)

func TestNormalizeCredentialPath(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    string
		wantErr string
	}{
		{name: "trim slashes and spaces", input: " /team/prod/password/ ", want: "team/prod/password"},
		{name: "reject empty", input: "", wantErr: "required"},
		{name: "reject parent traversal", input: "team/../password", wantErr: "escape"},
		{name: "reject root traversal", input: "../password", wantErr: "escape"},
		{name: "reject nul", input: "team/secret\x00", wantErr: "NUL"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := normalizeCredentialPath(tt.input)
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("expected error containing %q, got %v", tt.wantErr, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("normalize path: %v", err)
			}
			if got != tt.want {
				t.Fatalf("expected %q, got %q", tt.want, got)
			}
		})
	}
}

func TestValidateCredentialPathRejectsUnsupportedProvider(t *testing.T) {
	service := NewService(nil, nil, nil)
	if err := service.ValidateCredentialPath("unknown", "team/password"); err == nil {
		t.Fatal("expected unsupported provider error")
	}
}

func TestValidateCredentialPathRejectsInvalidPathBeforeProviderAccess(t *testing.T) {
	service := NewService(nil, nil, nil)
	for _, input := range []string{"", "../password", "team/../password", "\x00"} {
		if err := service.ValidateCredentialPath("vault", input); err == nil {
			t.Fatalf("expected validation error for %q", input)
		}
	}
}
