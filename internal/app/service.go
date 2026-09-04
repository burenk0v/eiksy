package app

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"opsy/internal/domain/ai"
	"opsy/internal/domain/credentials"
	"opsy/internal/domain/protocols"
	"opsy/internal/domain/sessions"
	"opsy/internal/domain/settings"
	sftpdomain "opsy/internal/domain/sftp"
	vaultdomain "opsy/internal/domain/vault"
	"opsy/internal/domain/workspace"
	"opsy/internal/llm"

	"github.com/tobischo/gokeepasslib/v3"
)

type ShellState struct {
	Protocols           []protocols.Descriptor           `json:"protocols"`
	SessionProfiles     []sessions.Profile               `json:"sessionProfiles"`
	ActiveSessions      []RuntimeSessionView             `json:"activeSessions"`
	SessionHistory      []sessions.HistoryEntry          `json:"sessionHistory"`
	CredentialProviders []credentials.ProviderDescriptor `json:"credentialProviders"`
	AI                  ai.WorkspaceState                `json:"ai"`
	Workspace           WorkspaceView                    `json:"workspace"`
	Settings            settings.AppSettings             `json:"settings"`
}

type WorkspaceView struct {
	Layout       workspace.Layout  `json:"layout"`
	RecentEvents []workspace.Event `json:"recentEvents"`
}

type RuntimeSessionView struct {
	ID          string `json:"id"`
	Title       string `json:"title"`
	ProtocolID  string `json:"protocolId"`
	ProfileID   string `json:"profileId"`
	Status      string `json:"status"`
	Description string `json:"description"`
}

type Service struct {
	ctx          context.Context
	store        stateStore
	localManager localModelManager
	sshManager   sshManager
	sftpManager  sftpManager
	httpClient   *http.Client
	emitFn       func(eventName string, data ...interface{})
	logMu        sync.Mutex
	logFilePath  string
}

type stateStore interface {
	Protocols() []protocols.Descriptor
	SessionProfiles() []sessions.Profile
	SessionProfile(string) (sessions.Profile, bool)
	LaunchHistory() []sessions.HistoryEntry
	CredentialProviders() []credentials.ProviderDescriptor
	AIState() ai.WorkspaceState
	Settings() settings.AppSettings
	WorkspaceLayout() workspace.Layout
	RuntimeTabs() []workspace.Tab
	Events() []workspace.Event
	UpdateAIState(ai.WorkspaceState)
	UpdateSettings(settings.AppSettings) error
	OpenRuntimeTab(workspace.Tab)
	CloseRuntimeTab(string) bool
	RecordLaunch(string)
}

type sessionProfileMutator interface {
	UpsertSessionProfile(sessions.Profile) error
	DeleteSessionProfile(string) error
}

type localModelManager interface {
	DownloadQwen3Model(context.Context) (string, error)
	DownloadQwen3ModelWithProgress(context.Context, func(downloaded, total int64)) (string, error)
	StartLocalServer(context.Context, string) (string, string, error)
}

type sshManager interface {
	Connect(context.Context, string, string, int, string, string, map[string]string) error
	SendInput(string, string) error
	ResizeTerminal(string, int, int) error
	Disconnect(string) error
	SetOutputHandler(string, func(data string))
	GetCurrentDir(string) (string, error)
	AcceptHostKey(string) error
}

type sftpManager interface {
	Connect(context.Context, string, string, int, string, string, map[string]string) error
	Connected(string) bool
	ListDir(string, string) ([]sftpdomain.FileEntry, error)
	ReadFile(string, string) (string, error)
	WriteFile(string, string, string) error
	UploadFile(string, string, string) error
	DownloadFile(string, string, string) error
	Disconnect(string) error
}

func NewService(store stateStore, localManager localModelManager, sshManager sshManager, sftpManager sftpManager) *Service {
	service := &Service{
		store:        store,
		localManager: localManager,
		sshManager:   sshManager,
		sftpManager:  sftpManager,
		httpClient:   &http.Client{Timeout: 120 * time.Second},
		emitFn:       func(string, ...interface{}) {},
	}
	if baseDir, err := llm.OpsyDir(); err == nil {
		service.logFilePath = filepath.Join(baseDir, "opsy.log")
	}
	return service
}

func (s *Service) SetRuntimeContext(ctx context.Context, emitFn func(eventName string, data ...interface{})) {
	s.ctx = ctx
	if emitFn != nil {
		s.emitFn = emitFn
	}
}

func (s *Service) EmitLog(level, message string) {
	timestamp := time.Now().UTC().Format(time.RFC3339)
	s.emitFn("app:log", map[string]string{
		"level":   level,
		"message": message,
		"time":    timestamp,
	})
	_ = s.writeLogToFile(level, message, timestamp)
}

func (s *Service) GetShellState() ShellState {
	tabs := s.store.RuntimeTabs()
	activeSessions := make([]RuntimeSessionView, 0, len(tabs))
	for _, tab := range tabs {
		activeSessions = append(activeSessions, RuntimeSessionView(tab))
	}

	return ShellState{
		Protocols:           s.store.Protocols(),
		SessionProfiles:     s.store.SessionProfiles(),
		ActiveSessions:      activeSessions,
		SessionHistory:      s.store.LaunchHistory(),
		CredentialProviders: s.store.CredentialProviders(),
		AI:                  s.store.AIState(),
		Workspace: WorkspaceView{
			Layout:       s.store.WorkspaceLayout(),
			RecentEvents: s.store.Events(),
		},
		Settings: s.store.Settings(),
	}
}

func (s *Service) CreateSessionProfile(profile sessions.Profile) error {
	mutator, ok := s.store.(sessionProfileMutator)
	if !ok {
		return fmt.Errorf("session profiles are read-only")
	}

	profile = normalizeProfile(profile)
	if err := s.validateProfile(profile); err != nil {
		s.EmitLog("warn", fmt.Sprintf("Session profile validation failed: %v", err))
		return err
	}
	if profile.ID == "" {
		profile.ID = s.nextProfileID(profile.Name)
	}
	if existing, found := s.store.SessionProfile(profile.ID); found {
		if profile.LastLaunchedAt == "" {
			profile.LastLaunchedAt = existing.LastLaunchedAt
		}
		if string(profile.Password) == "" {
			profile.Password = existing.Password
		}
	}

	if err := mutator.UpsertSessionProfile(profile); err != nil {
		s.EmitLog("error", fmt.Sprintf("Failed to save session profile %q: %v", profile.Name, err))
		return err
	}
	s.EmitLog("info", fmt.Sprintf("Session profile %q saved", profile.Name))
	return nil
}

func (s *Service) DeleteSessionProfile(id string) error {
	mutator, ok := s.store.(sessionProfileMutator)
	if !ok {
		return fmt.Errorf("session profiles are read-only")
	}
	id = strings.TrimSpace(id)
	if id == "" {
		return fmt.Errorf("session profile id is required")
	}
	return mutator.DeleteSessionProfile(id)
}

func (s *Service) LaunchSession(profileID string) (RuntimeSessionView, error) {
	profile, ok := s.store.SessionProfile(profileID)
	if !ok {
		return RuntimeSessionView{}, fmt.Errorf("session profile %q not found", profileID)
	}

	tab := workspace.Tab{
		ID:          fmt.Sprintf("%s-%d", profile.ProtocolID, time.Now().UTC().UnixNano()),
		Title:       profile.Name,
		ProtocolID:  profile.ProtocolID,
		ProfileID:   profile.ID,
		Status:      "connecting",
		Description: fmt.Sprintf("%s / %s@%s:%d", profile.ProtocolID, profile.Username, profile.Host, profile.Port),
	}

	s.store.OpenRuntimeTab(tab)
	s.store.RecordLaunch(profile.ID)

	return RuntimeSessionView(tab), nil
}

func (s *Service) CloseSession(sessionID string) error {
	if ok := s.store.CloseRuntimeTab(sessionID); !ok {
		return fmt.Errorf("active session %q not found", sessionID)
	}
	if s.sshManager != nil {
		_ = s.sshManager.Disconnect(sessionID)
	}
	if s.sftpManager != nil {
		_ = s.sftpManager.Disconnect(sessionID)
	}
	return nil
}

func (s *Service) ConnectSSH(ctx context.Context, tabID, profileID string) error {
	if s.sshManager == nil {
		return fmt.Errorf("ssh manager is not configured")
	}
	profile, ok := s.store.SessionProfile(profileID)
	if !ok {
		return fmt.Errorf("session profile %q not found", profileID)
	}
	if profile.ProtocolID != "ssh" {
		return fmt.Errorf("session profile %q does not use ssh", profileID)
	}

	s.sshManager.SetOutputHandler(tabID, func(data string) {
		s.emitFn(fmt.Sprintf("terminal:output:%s", tabID), map[string]string{"data": data})
	})
	_ = s.updateTabStatus(tabID, "connecting")
	profile = s.applySSHForwardingSettings(profile)
	s.EmitLog("info", fmt.Sprintf("Connecting SSH to %s@%s:%d", profile.Username, profile.Host, profile.Port))
	if err := s.sshManager.Connect(s.resolveContext(ctx), tabID, profile.Host, profile.Port, profile.Username, string(profile.Password), profile.Options); err != nil {
		s.EmitLog("error", fmt.Sprintf("SSH connection to %s@%s:%d failed: %v", profile.Username, profile.Host, profile.Port, err))
		_ = s.updateTabStatus(tabID, "error")
		return err
	}
	s.EmitLog("info", fmt.Sprintf("SSH connected to %s@%s:%d", profile.Username, profile.Host, profile.Port))
	return s.updateTabStatus(tabID, "connected")
}

func (s *Service) OpenRDP(tabID, profileID string) (string, error) {
	profile, ok := s.store.SessionProfile(profileID)
	if !ok {
		return "", fmt.Errorf("session profile %q not found", profileID)
	}
	if profile.ProtocolID != "rdp" {
		return "", fmt.Errorf("session profile %q does not use rdp", profileID)
	}
	if err := s.updateTabStatus(tabID, "connected"); err != nil {
		return "", err
	}
	return fmt.Sprintf("rdp://%s:%d", profile.Host, profile.Port), nil
}

func (s *Service) SendSSHInput(tabID, data string) error {
	if s.sshManager == nil {
		return fmt.Errorf("ssh manager is not configured")
	}
	return s.sshManager.SendInput(tabID, data)
}

func (s *Service) ResizeTerminal(tabID string, cols, rows int) error {
	if s.sshManager == nil {
		return fmt.Errorf("ssh manager is not configured")
	}
	return s.sshManager.ResizeTerminal(tabID, cols, rows)
}

func (s *Service) DisconnectSSH(tabID string) error {
	if s.sshManager == nil {
		return fmt.Errorf("ssh manager is not configured")
	}
	if err := s.sshManager.Disconnect(tabID); err != nil {
		return err
	}
	return s.updateTabStatus(tabID, "disconnected")
}

func (s *Service) ListSFTPFiles(tabID, targetPath string) ([]sftpdomain.FileEntry, error) {
	if s.sftpManager == nil {
		return nil, fmt.Errorf("sftp manager is not configured")
	}
	if err := s.ensureSFTPConnection(tabID); err != nil {
		return nil, err
	}
	pathToList := strings.TrimSpace(targetPath)
	if pathToList == "" && s.sshManager != nil {
		currentDir, err := s.sshManager.GetCurrentDir(tabID)
		if err == nil && strings.TrimSpace(currentDir) != "" {
			pathToList = strings.TrimSpace(currentDir)
		}
	}
	if pathToList == "" {
		pathToList = "."
	}
	return s.sftpManager.ListDir(tabID, pathToList)
}

func (s *Service) NavigateSFTP(tabID, targetPath string) ([]sftpdomain.FileEntry, error) {
	return s.ListSFTPFiles(tabID, targetPath)
}

func (s *Service) ReadSFTPFile(tabID, filePath string) (string, error) {
	if s.sftpManager == nil {
		return "", fmt.Errorf("sftp manager is not configured")
	}
	if err := s.ensureSFTPConnection(tabID); err != nil {
		return "", err
	}
	filePath = strings.TrimSpace(filePath)
	if filePath == "" {
		return "", fmt.Errorf("file path is required")
	}
	return s.sftpManager.ReadFile(tabID, filePath)
}

func (s *Service) SaveSFTPFile(tabID, filePath, content string) error {
	if s.sftpManager == nil {
		return fmt.Errorf("sftp manager is not configured")
	}
	if err := s.ensureSFTPConnection(tabID); err != nil {
		return err
	}
	filePath = strings.TrimSpace(filePath)
	if filePath == "" {
		return fmt.Errorf("file path is required")
	}
	return s.sftpManager.WriteFile(tabID, filePath, content)
}

func (s *Service) UploadSFTPFiles(tabID, remoteDir string, localPaths []string) error {
	if s.sftpManager == nil {
		return fmt.Errorf("sftp manager is not configured")
	}
	if err := s.ensureSFTPConnection(tabID); err != nil {
		return err
	}
	remoteDir = strings.TrimSpace(remoteDir)
	if remoteDir == "" {
		return fmt.Errorf("remote directory is required")
	}
	if len(localPaths) == 0 {
		return fmt.Errorf("at least one local file is required")
	}
	for _, localPath := range localPaths {
		localPath = strings.TrimSpace(localPath)
		if localPath == "" {
			continue
		}
		remotePath := path.Join(remoteDir, filepath.Base(localPath))
		if err := s.sftpManager.UploadFile(tabID, localPath, remotePath); err != nil {
			return err
		}
	}
	return nil
}

func (s *Service) DownloadSFTPFiles(tabID, localDir string, remotePaths []string) error {
	if s.sftpManager == nil {
		return fmt.Errorf("sftp manager is not configured")
	}
	if err := s.ensureSFTPConnection(tabID); err != nil {
		return err
	}
	localDir = strings.TrimSpace(localDir)
	if localDir == "" {
		return fmt.Errorf("local directory is required")
	}
	if len(remotePaths) == 0 {
		return fmt.Errorf("at least one remote file is required")
	}
	for _, remotePath := range remotePaths {
		remotePath = strings.TrimSpace(remotePath)
		if remotePath == "" {
			continue
		}
		localPath := filepath.Join(localDir, path.Base(remotePath))
		if err := s.sftpManager.DownloadFile(tabID, remotePath, localPath); err != nil {
			return err
		}
	}
	return nil
}

func (s *Service) ListVaultSecrets(path string) ([]vaultdomain.SecretNode, error) {
	return s.ListVaultSecretsForProvider("", path)
}

func (s *Service) ListVaultSecretsForProvider(provider, path string) ([]vaultdomain.SecretNode, error) {
	cfg := s.store.Settings()
	selectedProvider := strings.ToLower(strings.TrimSpace(provider))
	if selectedProvider == "" {
		selectedProvider = strings.ToLower(strings.TrimSpace(cfg.VaultProvider))
	}
	if selectedProvider == "" {
		selectedProvider = settings.DefaultVaultProvider
	}
	if selectedProvider != "vault" && selectedProvider != "keepass" {
		return nil, fmt.Errorf("unsupported vault provider %q", selectedProvider)
	}
	if selectedProvider == "keepass" {
		return s.listKeePassSecrets(path, cfg.KeePassDatabasePath, cfg.KeePassPassword)
	}
	if strings.TrimSpace(cfg.VaultAddress) == "" {
		return nil, fmt.Errorf("vault address is not configured")
	}
	if strings.TrimSpace(cfg.VaultMountPoint) == "" {
		return nil, fmt.Errorf("vault mountpoint is not configured")
	}
	if strings.TrimSpace(cfg.VaultToken) == "" {
		return nil, fmt.Errorf("vault token is not configured")
	}
	parsed, err := url.Parse(strings.TrimSpace(cfg.VaultAddress))
	if err != nil || parsed.Host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return nil, fmt.Errorf("vault address must be a valid http or https url")
	}
	if cfg.VaultAutoRenewToken {
		if err := s.renewVaultToken(parsed, cfg.VaultToken); err != nil {
			s.EmitLog("warn", fmt.Sprintf("Vault token auto-renew failed: %v", err))
		}
	}

	cleanPath := strings.Trim(strings.TrimSpace(path), "/")
	metadataRoute := vaultPathJoin(cfg.VaultMountPoint, "metadata", cleanPath)
	endpoint := strings.TrimRight(parsed.String(), "/") + "/v1/" + metadataRoute + "?list=true"
	req, err := http.NewRequestWithContext(s.resolveContext(nil), http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("build vault request: %w", err)
	}
	req.Header.Set("X-Vault-Token", cfg.VaultToken)

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("vault request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read vault response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("vault API returned %s: %s", resp.Status, strings.TrimSpace(string(body)))
	}

	var payload struct {
		Data struct {
			Keys []string `json:"keys"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, fmt.Errorf("parse vault response: %w", err)
	}

	result := make([]vaultdomain.SecretNode, 0, len(payload.Data.Keys))
	for _, key := range payload.Data.Keys {
		isDir := strings.HasSuffix(key, "/")
		name := strings.TrimSuffix(strings.TrimSpace(key), "/")
		if name == "" {
			continue
		}
		fullPath := name
		if cleanPath != "" {
			fullPath = cleanPath + "/" + name
		}
		result = append(result, vaultdomain.SecretNode{
			Name:  name,
			Path:  fullPath,
			IsDir: isDir,
		})
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].IsDir != result[j].IsDir {
			return result[i].IsDir
		}
		return result[i].Name < result[j].Name
	})
	return result, nil
}

func (s *Service) listKeePassSecrets(targetPath, dbPath, password string) ([]vaultdomain.SecretNode, error) {
	dbPath = strings.TrimSpace(dbPath)
	password = strings.TrimSpace(password)
	if dbPath == "" {
		return nil, fmt.Errorf("keepass database path is not configured")
	}
	if strings.HasPrefix(dbPath, "~/") {
		if homeDir, err := os.UserHomeDir(); err == nil && homeDir != "" {
			dbPath = filepath.Join(homeDir, strings.TrimPrefix(dbPath, "~/"))
		}
	}
	if password == "" {
		return nil, fmt.Errorf("keepass password is not configured")
	}
	file, err := os.Open(dbPath)
	if err != nil {
		return nil, fmt.Errorf("open keepass database: %w", err)
	}
	defer file.Close()

	database := gokeepasslib.NewDatabase()
	database.Credentials = gokeepasslib.NewPasswordCredentials(password)
	if err := gokeepasslib.NewDecoder(file).Decode(database); err != nil {
		return nil, fmt.Errorf("decode keepass database: %w", err)
	}
	if err := database.UnlockProtectedEntries(); err != nil {
		return nil, fmt.Errorf("unlock keepass database entries: %w", err)
	}
	if len(database.Content.Root.Groups) == 0 {
		return nil, fmt.Errorf("keepass database does not contain root groups")
	}

	cleanPath := strings.Trim(strings.TrimSpace(targetPath), "/")
	pathParts := []string{}
	if cleanPath != "" {
		pathParts = strings.Split(cleanPath, "/")
	}
	group := &database.Content.Root.Groups[0]
	for _, part := range pathParts {
		found := false
		for index := range group.Groups {
			name := strings.TrimSpace(group.Groups[index].Name)
			if name == part {
				group = &group.Groups[index]
				found = true
				break
			}
		}
		if !found {
			return nil, fmt.Errorf("keepass path %q not found", cleanPath)
		}
	}

	result := make([]vaultdomain.SecretNode, 0, len(group.Groups)+len(group.Entries))
	for _, child := range group.Groups {
		name := strings.TrimSpace(child.Name)
		if name == "" {
			continue
		}
		fullPath := name
		if cleanPath != "" {
			fullPath = cleanPath + "/" + name
		}
		result = append(result, vaultdomain.SecretNode{Name: name, Path: fullPath, IsDir: true})
	}
	for _, entry := range group.Entries {
		title := strings.TrimSpace(entry.GetTitle())
		if title == "" {
			continue
		}
		fullPath := title
		if cleanPath != "" {
			fullPath = cleanPath + "/" + title
		}
		result = append(result, vaultdomain.SecretNode{Name: title, Path: fullPath, IsDir: false})
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].IsDir != result[j].IsDir {
			return result[i].IsDir
		}
		return result[i].Name < result[j].Name
	})
	return result, nil
}

func (s *Service) renewVaultToken(baseURL *url.URL, token string) error {
	endpoint := strings.TrimRight(baseURL.String(), "/") + "/v1/auth/token/renew-self"
	req, err := http.NewRequestWithContext(s.resolveContext(nil), http.MethodPost, endpoint, bytes.NewReader([]byte(`{}`)))
	if err != nil {
		return fmt.Errorf("build vault token renewal request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Vault-Token", token)

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("vault token renewal failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read vault token renewal response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("vault token renewal returned %s: %s", resp.Status, strings.TrimSpace(string(body)))
	}
	return nil
}

func (s *Service) SelectAIProvider(providerID string) error {
	state := s.store.AIState()
	index := providerIndexByID(state.Providers, providerID)
	if index < 0 {
		return fmt.Errorf("ai provider %q not found", providerID)
	}

	for i := range state.Providers {
		state.Providers[i].Selected = state.Providers[i].ID == providerID
	}

	s.store.UpdateAIState(state)
	s.EmitLog("info", fmt.Sprintf("Switched AI provider to %s.", state.Providers[index].Name))
	return nil
}

func (s *Service) SaveCloudProvider(model, endpoint, token string) error {
	model = strings.TrimSpace(model)
	endpoint = strings.TrimSpace(endpoint)
	token = strings.TrimSpace(token)
	if model == "" {
		return fmt.Errorf("model name is required")
	}
	if endpoint == "" {
		return fmt.Errorf("cloud endpoint is required")
	}

	parsed, err := url.Parse(endpoint)
	if err != nil || parsed.Host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return fmt.Errorf("cloud endpoint must be a valid http or https url")
	}

	state := s.store.AIState()
	index := providerIndexByClass(state.Providers, ai.ProviderClassOpenAICompatible)
	if index < 0 {
		return fmt.Errorf("cloud ai provider is not available")
	}

	for i := range state.Providers {
		state.Providers[i].Selected = i == index
	}

	state.Providers[index].Model = model
	state.Providers[index].Endpoint = endpoint
	if token == "" {
		token = state.Providers[index].Token
	}
	state.Providers[index].Token = token
	state.Providers[index].Status = "ready"
	state.Providers[index].Configured = true
	s.store.UpdateAIState(state)
	s.EmitLog("info", fmt.Sprintf("Saved cloud AI provider %s (%s).", state.Providers[index].Name, model))
	return nil
}

func (s *Service) ListCloudModels(endpoint, token string) ([]string, error) {
	endpoint = strings.TrimSpace(endpoint)
	token = strings.TrimSpace(token)
	if endpoint == "" {
		return nil, fmt.Errorf("cloud endpoint is required")
	}

	parsed, err := url.Parse(endpoint)
	if err != nil || parsed.Host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return nil, fmt.Errorf("cloud endpoint must be a valid http or https url")
	}

	if token == "" {
		state := s.store.AIState()
		if index := providerIndexByClass(state.Providers, ai.ProviderClassOpenAICompatible); index >= 0 {
			token = strings.TrimSpace(state.Providers[index].Token)
		}
	}

	req, err := http.NewRequestWithContext(s.resolveContext(nil), http.MethodGet, strings.TrimRight(endpoint, "/")+"/models", nil)
	if err != nil {
		return nil, err
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("AI API returned %s: %s", resp.Status, strings.TrimSpace(string(respBody)))
	}

	var payload struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := json.Unmarshal(respBody, &payload); err != nil {
		return nil, fmt.Errorf("parse model list response: %w", err)
	}

	unique := map[string]struct{}{}
	models := make([]string, 0, len(payload.Data))
	for _, entry := range payload.Data {
		id := strings.TrimSpace(entry.ID)
		if id == "" {
			continue
		}
		if _, exists := unique[id]; exists {
			continue
		}
		unique[id] = struct{}{}
		models = append(models, id)
	}
	sort.Strings(models)
	return models, nil
}

func (s *Service) UpdateSettings(updated settings.AppSettings) error {
	current := s.store.Settings()
	// Preserve fields that are not exposed in the update call.
	if updated.Theme == "" {
		updated.Theme = current.Theme
	}
	if updated.DefaultProtocol == "" {
		updated.DefaultProtocol = current.DefaultProtocol
	}
	if updated.WindowLayout.SidebarWidth == 0 {
		updated.WindowLayout.SidebarWidth = current.WindowLayout.SidebarWidth
	}
	if updated.WindowLayout.AssistantWidth == 0 {
		updated.WindowLayout.AssistantWidth = current.WindowLayout.AssistantWidth
	}
	if updated.LogLevel == "" {
		updated.LogLevel = current.LogLevel
	}
	if updated.LogRotationSize <= 0 {
		updated.LogRotationSize = current.LogRotationSize
	}
	if updated.LogRotationSize <= 0 {
		updated.LogRotationSize = settings.DefaultLogRotationSize
	}
	if strings.TrimSpace(updated.VaultMountPoint) == "" {
		updated.VaultMountPoint = current.VaultMountPoint
	}
	if strings.TrimSpace(updated.VaultMountPoint) == "" {
		updated.VaultMountPoint = settings.DefaultVaultMountPoint
	}
	if strings.TrimSpace(updated.VaultProvider) == "" {
		updated.VaultProvider = current.VaultProvider
	}
	if strings.TrimSpace(updated.VaultProvider) == "" {
		updated.VaultProvider = settings.DefaultVaultProvider
	}
	updated.VaultProvider = strings.ToLower(strings.TrimSpace(updated.VaultProvider))
	if updated.VaultProvider != "vault" && updated.VaultProvider != "keepass" {
		updated.VaultProvider = settings.DefaultVaultProvider
	}
	if strings.TrimSpace(updated.KeePassPassword) == "" {
		updated.KeePassPassword = current.KeePassPassword
	}
	if strings.TrimSpace(updated.VaultToken) == "" {
		updated.VaultToken = current.VaultToken
	}
	updated.VaultAddress = strings.TrimSpace(updated.VaultAddress)
	updated.VaultMountPoint = strings.Trim(strings.TrimSpace(updated.VaultMountPoint), "/")
	updated.KeePassDatabasePath = strings.TrimSpace(updated.KeePassDatabasePath)
	updated.KeePassPassword = strings.TrimSpace(updated.KeePassPassword)
	updated.VaultToken = strings.TrimSpace(updated.VaultToken)
	updated.SSHForwardPorts = sanitizeForwardPorts(updated.SSHForwardPorts)
	updated.SSHForwardHostID = strings.TrimSpace(updated.SSHForwardHostID)
	updated.PortForwardRules = normalizeForwardRules(updated.PortForwardRules)
	return s.store.UpdateSettings(updated)
}

func (s *Service) DownloadLocalModel(ctx context.Context) error {
	return s.DownloadLocalModelWithProgress(ctx, nil)
}

func (s *Service) DownloadLocalModelWithProgress(ctx context.Context, progressFn func(downloaded, total int64)) error {
	if s.localManager == nil {
		return fmt.Errorf("local model manager is not configured")
	}

	modelPath, err := s.localManager.DownloadQwen3ModelWithProgress(s.resolveContext(ctx), progressFn)
	if err != nil {
		return err
	}

	state := s.store.AIState()
	index := providerIndexByClass(state.Providers, ai.ProviderClassLocal)
	if index < 0 {
		return fmt.Errorf("local ai provider is not available")
	}

	for i := range state.Providers {
		state.Providers[i].Selected = i == index
	}

	state.Providers[index].LocalPath = modelPath
	state.Providers[index].Status = "downloaded"
	state.Providers[index].Configured = false
	s.store.UpdateAIState(state)
	s.EmitLog("info", "Downloaded the Qwen3 8B local model.")
	return nil
}

func (s *Service) StartLocalModel(ctx context.Context) error {
	if s.localManager == nil {
		return fmt.Errorf("local model manager is not configured")
	}

	state := s.store.AIState()
	index := providerIndexByClass(state.Providers, ai.ProviderClassLocal)
	if index < 0 {
		return fmt.Errorf("local ai provider is not available")
	}

	endpoint, command, err := s.localManager.StartLocalServer(s.resolveContext(ctx), state.Providers[index].LocalPath)
	if err != nil {
		return err
	}

	for i := range state.Providers {
		state.Providers[i].Selected = i == index
	}

	state.Providers[index].Endpoint = endpoint
	state.Providers[index].Command = command
	state.Providers[index].Status = "running via llama.cpp"
	state.Providers[index].Configured = true
	s.store.UpdateAIState(state)
	s.EmitLog("info", "Started the Qwen3 8B local model with llama.cpp.")
	return nil
}

func (s *Service) ensureSFTPConnection(tabID string) error {
	tab, ok := s.runtimeTab(tabID)
	if !ok {
		return fmt.Errorf("active session %q not found", tabID)
	}
	profile, ok := s.store.SessionProfile(tab.ProfileID)
	if !ok {
		return fmt.Errorf("session profile %q not found", tab.ProfileID)
	}
	if s.sftpManager.Connected(tabID) {
		return nil
	}
	return s.sftpManager.Connect(s.resolveContext(nil), tabID, profile.Host, profile.Port, profile.Username, string(profile.Password), profile.Options)
}

func (s *Service) runtimeTab(tabID string) (workspace.Tab, bool) {
	for _, tab := range s.store.RuntimeTabs() {
		if tab.ID == tabID {
			return tab, true
		}
	}
	return workspace.Tab{}, false
}

func (s *Service) updateTabStatus(tabID, status string) error {
	tab, ok := s.runtimeTab(tabID)
	if !ok {
		return fmt.Errorf("active session %q not found", tabID)
	}
	tab.Status = status
	s.store.OpenRuntimeTab(tab)
	return nil
}

func (s *Service) validateProfile(profile sessions.Profile) error {
	if strings.TrimSpace(profile.Name) == "" {
		return fmt.Errorf("session name is required")
	}
	if strings.TrimSpace(profile.ProtocolID) == "" {
		return fmt.Errorf("protocol is required")
	}
	if strings.TrimSpace(profile.Host) == "" {
		return fmt.Errorf("host is required")
	}
	if strings.TrimSpace(profile.Username) == "" {
		return fmt.Errorf("username is required")
	}
	if profile.Port <= 0 {
		return fmt.Errorf("port must be greater than 0")
	}
	if !s.protocolExists(profile.ProtocolID) {
		return fmt.Errorf("protocol %q is not supported", profile.ProtocolID)
	}
	return nil
}

func (s *Service) protocolExists(protocolID string) bool {
	for _, protocol := range s.store.Protocols() {
		if protocol.ID == protocolID {
			return true
		}
	}
	return false
}

var nonSlugPattern = regexp.MustCompile(`[^a-z0-9]+`)

func (s *Service) nextProfileID(name string) string {
	base := strings.Trim(nonSlugPattern.ReplaceAllString(strings.ToLower(name), "-"), "-")
	if base == "" {
		base = "session"
	}
	candidate := base
	counter := 2
	for {
		if _, exists := s.store.SessionProfile(candidate); !exists {
			return candidate
		}
		candidate = fmt.Sprintf("%s-%d", base, counter)
		counter++
	}
}

func normalizeProfile(profile sessions.Profile) sessions.Profile {
	profile.ID = strings.TrimSpace(profile.ID)
	profile.Name = strings.TrimSpace(profile.Name)
	profile.Group = ""
	profile.ProtocolID = strings.TrimSpace(strings.ToLower(profile.ProtocolID))
	profile.Host = strings.TrimSpace(profile.Host)
	profile.Username = strings.TrimSpace(profile.Username)
	profile.Password = sessions.EncryptedString(strings.TrimSpace(string(profile.Password)))
	if profile.Port <= 0 {
		profile.Port = 22
	}
	profile.Tags = normalizeTags(profile.Tags)
	return profile
}

func (s *Service) applySSHForwardingSettings(profile sessions.Profile) sessions.Profile {
	if profile.ProtocolID != "ssh" {
		return profile
	}
	cfg := s.store.Settings()

	// Build the combined list of active rules from PortForwardRules.
	// Fall back to the legacy SSHForwardPorts/SSHForwardHostID fields when
	// no explicit rules are configured so that existing settings keep working.
	type ruleEntry struct {
		localPorts string
		remoteHost string
		remotePort string
		hostID     string
	}
	var rules []ruleEntry
	for _, r := range cfg.PortForwardRules {
		if !r.Enabled {
			continue
		}
		p := strings.TrimSpace(r.LocalPort)
		if p == "" {
			p = strings.TrimSpace(r.Ports)
		}
		rh := strings.TrimSpace(r.RemoteHost)
		rp := strings.TrimSpace(r.RemotePort)
		h := strings.TrimSpace(r.HostID)
		if p != "" && h != "" && rh != "" {
			rules = append(rules, ruleEntry{localPorts: p, remoteHost: rh, remotePort: rp, hostID: h})
		}
	}
	if len(rules) == 0 {
		legacyPorts := strings.TrimSpace(cfg.SSHForwardPorts)
		legacyHostID := strings.TrimSpace(cfg.SSHForwardHostID)
		if legacyPorts != "" && legacyHostID != "" {
			rules = append(rules, ruleEntry{localPorts: legacyPorts, remoteHost: "", remotePort: "", hostID: legacyHostID})
		}
	}
	if len(rules) == 0 {
		return profile
	}

	allSpecs := make([]string, 0)
	for _, rule := range rules {
		targetProfile, ok := s.store.SessionProfile(rule.hostID)
		if !ok || targetProfile.ProtocolID != "ssh" || strings.TrimSpace(targetProfile.Host) == "" {
			continue
		}
		remoteHost := rule.remoteHost
		if remoteHost == "" {
			remoteHost = strings.TrimSpace(targetProfile.Host)
		}
		remotePort := rule.remotePort
		spec := buildPortForwardSpecs(rule.localPorts, remoteHost, remotePort)
		if strings.TrimSpace(spec) != "" {
			allSpecs = append(allSpecs, spec)
		}
	}
	if len(allSpecs) == 0 {
		return profile
	}

	options := make(map[string]string, len(profile.Options)+1)
	for key, value := range profile.Options {
		options[key] = value
	}
	options["local_forwards"] = strings.Join(allSpecs, ",")
	profile.Options = options
	return profile
}

func buildPortForwardSpecs(localPorts, remoteHost, remotePort string) string {
	const maxRangeSpan = 256
	remoteHost = strings.TrimSpace(remoteHost)
	remotePort = strings.TrimSpace(remotePort)
	if remoteHost == "" {
		return ""
	}
	remotePortValue := 0
	if remotePort != "" {
		parsedRemotePort, err := strconv.Atoi(remotePort)
		if err != nil || parsedRemotePort <= 0 || parsedRemotePort > 65535 {
			return ""
		}
		remotePortValue = parsedRemotePort
	}
	parts := strings.Split(localPorts, ",")
	specs := make([]string, 0, len(parts))
	for _, part := range parts {
		token := strings.TrimSpace(part)
		if token == "" {
			continue
		}
		if strings.Contains(token, "-") {
			rangeParts := strings.SplitN(token, "-", 2)
			start := strings.TrimSpace(rangeParts[0])
			end := strings.TrimSpace(rangeParts[1])
			startPort, errStart := strconv.Atoi(start)
			endPort, errEnd := strconv.Atoi(end)
			if errStart == nil && errEnd == nil && startPort > 0 && endPort >= startPort && endPort <= 65535 && (endPort-startPort+1) <= maxRangeSpan {
				for port := startPort; port <= endPort; port++ {
					targetPort := remotePortValue
					if targetPort == 0 {
						targetPort = port
					}
					specs = append(specs, fmt.Sprintf("%d:%s:%d", port, remoteHost, targetPort))
				}
			}
			continue
		}
		port, err := strconv.Atoi(token)
		if err == nil && port > 0 && port <= 65535 {
			targetPort := remotePortValue
			if targetPort == 0 {
				targetPort = port
			}
			specs = append(specs, fmt.Sprintf("%d:%s:%d", port, remoteHost, targetPort))
		}
	}
	return strings.Join(specs, ",")
}

func sanitizeForwardPorts(value string) string {
	parts := strings.Split(value, ",")
	normalized := make([]string, 0, len(parts))
	for _, part := range parts {
		token := strings.TrimSpace(part)
		if token == "" {
			continue
		}
		normalized = append(normalized, token)
	}
	return strings.Join(normalized, ",")
}

func normalizeForwardRules(rules []settings.PortForwardRule) []settings.PortForwardRule {
	normalized := make([]settings.PortForwardRule, 0, len(rules))
	for _, rule := range rules {
		entry := rule
		entry.HostID = strings.TrimSpace(entry.HostID)
		entry.LocalPort = sanitizeForwardPorts(entry.LocalPort)
		if entry.LocalPort == "" {
			entry.LocalPort = sanitizeForwardPorts(entry.Ports)
		}
		entry.RemoteHost = strings.TrimSpace(entry.RemoteHost)
		entry.RemotePort = strings.TrimSpace(entry.RemotePort)
		normalized = append(normalized, entry)
	}
	return normalized
}

func normalizeTags(tags []string) []string {
	result := make([]string, 0, len(tags))
	seen := map[string]struct{}{}
	for _, tag := range tags {
		tag = strings.TrimSpace(tag)
		if tag == "" {
			continue
		}
		if _, ok := seen[tag]; ok {
			continue
		}
		seen[tag] = struct{}{}
		result = append(result, tag)
	}
	return result
}

func (s *Service) resolveContext(ctx context.Context) context.Context {
	if ctx != nil {
		return ctx
	}
	if s.ctx != nil {
		return s.ctx
	}
	return context.Background()
}

func providerIndexByID(providers []ai.ProviderDescriptor, providerID string) int {
	for index, provider := range providers {
		if provider.ID == providerID {
			return index
		}
	}

	return -1
}

func providerIndexByClass(providers []ai.ProviderDescriptor, class ai.ProviderClass) int {
	for index, provider := range providers {
		if provider.Class == class {
			return index
		}
	}

	return -1
}

// SendChatMessage adds the user message to the conversation, calls the
// configured AI provider, and appends the assistant reply.
func (s *Service) SendChatMessage(ctx context.Context, message string) error {
	message = strings.TrimSpace(message)
	if message == "" {
		return fmt.Errorf("message cannot be empty")
	}

	state := s.store.AIState()
	provider := s.activeConfiguredProvider(state)
	if provider == nil {
		return fmt.Errorf("no AI provider is configured; configure one in Settings first")
	}

	state.Messages = append(state.Messages, ai.ChatMessage{Role: "user", Content: message})
	s.store.UpdateAIState(state)

	s.emitFn("ai:status", map[string]string{"status": "thinking"})
	defer s.emitFn("ai:status", map[string]string{"status": "idle"})
	reply, err := s.callChatCompletion(s.resolveContext(ctx), provider, state.Messages, state.ChatSessionID)
	if err != nil {
		return fmt.Errorf("AI request failed: %w", err)
	}

	state = s.store.AIState()
	state.Messages = append(state.Messages, ai.ChatMessage{Role: "assistant", Content: reply})
	s.store.UpdateAIState(state)

	s.emitFn("ai:message", map[string]string{"role": "assistant", "content": reply})
	return nil
}

// ClearChat removes all messages from the AI chat history.
func (s *Service) ClearChat() {
	state := s.store.AIState()
	state.Messages = []ai.ChatMessage{}
	state.ChatSessionID = fmt.Sprintf("chat-%d", time.Now().UTC().UnixNano())
	s.store.UpdateAIState(state)
}

// AcceptSSHHostKey trusts the pending unknown host key for the given tab and
// adds it to the known_hosts file, so that a subsequent ConnectSSH succeeds.
func (s *Service) AcceptSSHHostKey(tabID string) error {
	if s.sshManager == nil {
		return fmt.Errorf("ssh manager is not configured")
	}
	return s.sshManager.AcceptHostKey(tabID)
}

func (s *Service) activeConfiguredProvider(state ai.WorkspaceState) *ai.ProviderDescriptor {
	for i := range state.Providers {
		if state.Providers[i].Selected && state.Providers[i].Configured {
			return &state.Providers[i]
		}
	}
	return nil
}

func (s *Service) callChatCompletion(ctx context.Context, provider *ai.ProviderDescriptor, messages []ai.ChatMessage, chatSessionID string) (string, error) {
	type reqMessage struct {
		Role    string `json:"role"`
		Content string `json:"content"`
	}
	reqMessages := make([]reqMessage, len(messages))
	for i, m := range messages {
		reqMessages[i] = reqMessage{Role: m.Role, Content: m.Content}
	}
	body, err := json.Marshal(map[string]interface{}{
		"model":    provider.Model,
		"messages": reqMessages,
		"user":     chatSessionID,
	})
	if err != nil {
		return "", err
	}

	endpoint := strings.TrimRight(provider.Endpoint, "/") + "/chat/completions"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	if provider.Token != "" {
		req.Header.Set("Authorization", "Bearer "+provider.Token)
	}

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("AI API returned %s: %s", resp.Status, strings.TrimSpace(string(respBody)))
	}

	var result struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return "", fmt.Errorf("parse AI response: %w", err)
	}
	if len(result.Choices) == 0 || result.Choices[0].Message.Content == "" {
		return "", fmt.Errorf("AI returned an empty response")
	}
	return result.Choices[0].Message.Content, nil
}

func (s *Service) writeLogToFile(level, message, timestamp string) error {
	cfg := s.store.Settings()
	if !cfg.SaveLogsToFile || strings.TrimSpace(s.logFilePath) == "" {
		return nil
	}

	s.logMu.Lock()
	defer s.logMu.Unlock()

	if err := os.MkdirAll(filepath.Dir(s.logFilePath), 0o755); err != nil {
		return err
	}
	line := fmt.Sprintf("%s [%s] %s\n", timestamp, strings.ToUpper(strings.TrimSpace(level)), message)
	maxSize := cfg.LogRotationSize
	if maxSize <= 0 {
		maxSize = settings.DefaultLogRotationSize
	}
	if err := s.rotateLogFileIfNeeded(maxSize, int64(len(line))); err != nil {
		return err
	}
	f, err := os.OpenFile(s.logFilePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = f.WriteString(line)
	return err
}

func (s *Service) rotateLogFileIfNeeded(maxSize int, incomingSize int64) error {
	info, err := os.Stat(s.logFilePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	if info.Size()+incomingSize <= int64(maxSize) {
		return nil
	}
	rotated := s.logFilePath + ".1"
	_ = os.Remove(rotated)
	return os.Rename(s.logFilePath, rotated)
}

func vaultPathJoin(parts ...string) string {
	filtered := make([]string, 0, len(parts))
	for _, part := range parts {
		p := strings.Trim(strings.TrimSpace(part), "/")
		if p == "" {
			continue
		}
		for _, token := range strings.Split(p, "/") {
			escaped := url.PathEscape(strings.TrimSpace(token))
			if escaped != "" {
				filtered = append(filtered, escaped)
			}
		}
	}
	return strings.Join(filtered, "/")
}
