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

// CreateSessionProfile is the shared application operation used by
// interactive clients. Secret fields are handled by CreateSessionProfileInput.
func (s *Service) CreateSessionProfile(input sessions.ProfileInput) error {
	return s.CreateSessionProfileInput(input)
}
