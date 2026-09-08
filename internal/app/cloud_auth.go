package app

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const cloudAuthSessionTimeout = 5 * time.Minute

type cloudAuthSession struct {
	state         CloudProviderAuthSession
	allowedOrigin string
	listener      net.Listener
	server        *http.Server
	timer         *time.Timer
}

func (s *Service) StartCloudProviderAuth(endpoint string) (CloudProviderAuthSession, error) {
	endpoint = strings.TrimSpace(endpoint)
	if endpoint == "" {
		return CloudProviderAuthSession{}, fmt.Errorf("cloud endpoint is required")
	}

	parsed, err := url.Parse(endpoint)
	if err != nil || parsed.Host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return CloudProviderAuthSession{}, fmt.Errorf("cloud endpoint must be a valid http or https url")
	}

	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		return CloudProviderAuthSession{}, fmt.Errorf("start local auth callback listener: %w", err)
	}
	port := listener.Addr().(*net.TCPAddr).Port
	origin := (&url.URL{Scheme: parsed.Scheme, Host: parsed.Host}).String()

	session := &cloudAuthSession{
		state: CloudProviderAuthSession{
			ID:       newCloudProviderAuthSessionID(),
			Status:   "pending",
			AuthURL:  fmt.Sprintf("%s/user/settings/tokens/new/callback?requestFrom=CODY_CLI-%d", origin, port),
			Message:  "Waiting for browser authorization.",
			Endpoint: endpoint,
		},
		allowedOrigin: origin,
		listener:      listener,
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/api/sourcegraph/token", func(w http.ResponseWriter, r *http.Request) {
		s.handleCloudProviderAuthCallback(session.state.ID, session.allowedOrigin, w, r)
	})
	session.server = &http.Server{
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}

	s.authMu.Lock()
	s.stopCloudAuthLocked()
	s.cloudAuth = session
	s.authMu.Unlock()

	session.timer = time.AfterFunc(cloudAuthSessionTimeout, func() {
		s.finishCloudProviderAuthSession(session.state.ID, "expired", "", "Browser authorization timed out.", true)
	})

	go func(server *http.Server, listener net.Listener, sessionID string) {
		if err := server.Serve(listener); err != nil && !errors.Is(err, http.ErrServerClosed) {
			s.finishCloudProviderAuthSession(sessionID, "failed", "", fmt.Sprintf("Browser authorization failed: %v", err), true)
		}
	}(session.server, listener, session.state.ID)

	return session.state, nil
}

func (s *Service) GetCloudProviderAuthSession(sessionID string) (CloudProviderAuthSession, error) {
	sessionID = strings.TrimSpace(sessionID)
	if sessionID == "" {
		return CloudProviderAuthSession{}, fmt.Errorf("cloud auth session id is required")
	}

	s.authMu.Lock()
	defer s.authMu.Unlock()

	if s.cloudAuth == nil || s.cloudAuth.state.ID != sessionID {
		return CloudProviderAuthSession{}, fmt.Errorf("cloud auth session %q not found", sessionID)
	}

	return s.cloudAuth.state, nil
}

func (s *Service) handleCloudProviderAuthCallback(sessionID, allowedOrigin string, w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodOptions:
		writeCloudProviderAuthCORSHeaders(w, allowedOrigin)
		w.WriteHeader(http.StatusOK)
		return
	case http.MethodPost:
		writeCloudProviderAuthCORSHeaders(w, allowedOrigin)
	case http.MethodGet:
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	token, err := readCloudProviderAuthToken(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if !s.finishCloudProviderAuthSession(sessionID, "completed", token, "Token received from browser authorization.", false) {
		http.Error(w, "authorization session is no longer active", http.StatusGone)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = io.WriteString(w, `<!doctype html><html><body><p>Authorization completed. You can close this window.</p></body></html>`)
	go s.closeCloudProviderAuthServer(sessionID)
}

func readCloudProviderAuthToken(r *http.Request) (string, error) {
	switch r.Method {
	case http.MethodGet:
		token := strings.TrimSpace(r.URL.Query().Get("token"))
		if token == "" {
			token = strings.TrimSpace(r.URL.Query().Get("code"))
		}
		if token == "" {
			return "", fmt.Errorf("token is missing in callback request")
		}
		return token, nil
	case http.MethodPost:
		defer r.Body.Close()
		var payload struct {
			AccessToken string `json:"accessToken"`
			Token       string `json:"token"`
			Code        string `json:"code"`
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			return "", fmt.Errorf("parse callback request: %w", err)
		}
		token := strings.TrimSpace(payload.AccessToken)
		if token == "" {
			token = strings.TrimSpace(payload.Token)
		}
		if token == "" {
			token = strings.TrimSpace(payload.Code)
		}
		if token == "" {
			return "", fmt.Errorf("token is missing in callback request")
		}
		return token, nil
	default:
		return "", fmt.Errorf("unsupported callback method")
	}
}

func writeCloudProviderAuthCORSHeaders(w http.ResponseWriter, allowedOrigin string) {
	if allowedOrigin == "" {
		return
	}
	w.Header().Set("Access-Control-Allow-Origin", allowedOrigin)
	w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
}

func (s *Service) finishCloudProviderAuthSession(sessionID, status, token, message string, stopServer bool) bool {
	var session *cloudAuthSession

	s.authMu.Lock()
	if s.cloudAuth == nil || s.cloudAuth.state.ID != sessionID {
		s.authMu.Unlock()
		return false
	}

	session = s.cloudAuth
	if status == "completed" && s.cloudAuth.state.Status == "completed" && s.cloudAuth.state.Token != "" {
		s.authMu.Unlock()
		return true
	}

	s.cloudAuth.state.Status = status
	s.cloudAuth.state.Message = message
	if token != "" {
		s.cloudAuth.state.Token = token
	}
	if session.timer != nil {
		session.timer.Stop()
		session.timer = nil
	}
	s.authMu.Unlock()

	if stopServer {
		s.stopCloudAuthServer(session)
	}
	return true
}

func (s *Service) closeCloudProviderAuthServer(sessionID string) {
	s.authMu.Lock()
	defer s.authMu.Unlock()
	if s.cloudAuth == nil || s.cloudAuth.state.ID != sessionID {
		return
	}
	session := s.cloudAuth
	if session.timer != nil {
		session.timer.Stop()
		session.timer = nil
	}
	go s.stopCloudAuthServer(session)
}

func (s *Service) stopCloudAuthLocked() {
	if s.cloudAuth == nil {
		return
	}
	session := s.cloudAuth
	if session.timer != nil {
		session.timer.Stop()
		session.timer = nil
	}
	s.cloudAuth = nil
	go s.stopCloudAuthServer(session)
}

func (s *Service) stopCloudAuthServer(session *cloudAuthSession) {
	if session == nil {
		return
	}
	if session.server != nil {
		_ = session.server.Close()
	}
	if session.listener != nil {
		_ = session.listener.Close()
	}
}

func newCloudProviderAuthSessionID() string {
	var data [12]byte
	if _, err := rand.Read(data[:]); err != nil {
		return fmt.Sprintf("cloud-auth-%d", time.Now().UTC().UnixNano())
	}
	return "cloud-auth-" + hex.EncodeToString(data[:])
}
