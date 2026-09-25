package app

import "eiksy/internal/domain/sessions"

// ListSessionProfiles exposes the persisted session profiles to interactive
// clients such as the TUI without exposing secret values.
func (s *Service) ListSessionProfiles() []sessions.Profile {
	if s == nil || s.store == nil {
		return nil
	}
	return s.store.SessionProfiles()
}
