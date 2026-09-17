package app

import (
	"context"
	"errors"
	"fmt"
	"path"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"eiksy/internal/domain/sessions"
	"eiksy/internal/domain/settings"
	sftpdomain "eiksy/internal/domain/sftp"
	"eiksy/internal/domain/workspace"
	"eiksy/internal/securestorage"
)

func (s *Service) CreateSessionProfileInput(input sessions.ProfileInput) error {
	profile := sessions.Profile{
		ID: input.ID, Name: input.Name, Group: input.Group, Tags: input.Tags,
		Favorite: input.Favorite, ProtocolID: input.ProtocolID, Host: input.Host,
		Port: input.Port, Username: input.Username, SecretRef: input.SecretRef,
		Options: input.Options, LastLaunchedAt: input.LastLaunchedAt,
	}
	if err := s.CreateSessionProfile(profile); err != nil {
		return err
	}
	if existing, ok := s.store.SessionProfile(profile.ID); ok {
		if strings.TrimSpace(input.Password) != "" {
			if err := s.store.StoreSecret(securestorage.SessionPasswordKey(existing.ID), strings.TrimSpace(input.Password)); err != nil { return err }
		}
		if strings.TrimSpace(input.KeyPassphrase) != "" {
			if err := s.store.StoreSecret(securestorage.SessionKeyPassphraseKey(existing.ID), strings.TrimSpace(input.KeyPassphrase)); err != nil { return err }
		}
		if strings.EqualFold(strings.TrimSpace(input.Options["auth_method"]), "key") {
			_ = s.store.DeleteSecret(securestorage.SessionPasswordKey(existing.ID))
		} else if strings.EqualFold(strings.TrimSpace(input.Options["auth_method"]), "password") {
			_ = s.store.DeleteSecret(securestorage.SessionKeyPassphraseKey(existing.ID))
		}
	}
	return nil
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
		if strings.TrimSpace(string(profile.Password)) == "" {
			profile.Password = existing.Password
		}
		if strings.TrimSpace(string(profile.KeyPassphrase)) == "" {
			profile.KeyPassphrase = existing.KeyPassphrase
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
	profile, err := s.profileWithSecrets(profile)
	if err != nil {
		return err
	}
	profile = s.applySSHForwardingSettings(profile)
	s.EmitLog("info", fmt.Sprintf("Connecting SSH to %s@%s:%d", profile.Username, profile.Host, profile.Port))
	if err := s.sshManager.Connect(s.resolveContext(ctx), tabID, profile.Host, profile.Port, profile.Username, profileCredential(profile), profile.Options); err != nil {
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
	profile, err := s.profileWithSecrets(profile)
	if err != nil {
		return err
	}
	return s.sftpManager.Connect(s.resolveContext(nil), tabID, profile.Host, profile.Port, profile.Username, profileCredential(profile), profile.Options)
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
	profile.KeyPassphrase = sessions.EncryptedString(strings.TrimSpace(string(profile.KeyPassphrase)))
	profile.Options = cloneProfileOptionsWithoutCredentialSecrets(profile.Options)
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

// AcceptSSHHostKey trusts the pending unknown host key for the given tab and
// adds it to the known_hosts file, so that a subsequent ConnectSSH succeeds.
func (s *Service) AcceptSSHHostKey(tabID string) error {
	if s.sshManager == nil {
		return fmt.Errorf("ssh manager is not configured")
	}
	return s.sshManager.AcceptHostKey(tabID)
}
func (s *Service) profileWithSecrets(profile sessions.Profile) (sessions.Profile, error) {
	password, err := s.store.LoadSecret(securestorage.SessionPasswordKey(profile.ID))
	if err != nil && !errors.Is(err, securestorage.ErrMasterPasswordRequired) {
		return sessions.Profile{}, err
	}
	keyPassphrase, err := s.store.LoadSecret(securestorage.SessionKeyPassphraseKey(profile.ID))
	if err != nil && !errors.Is(err, securestorage.ErrMasterPasswordRequired) {
		return sessions.Profile{}, err
	}
	cloned := cloneSessionProfile(profile)
	if strings.TrimSpace(password) != "" {
		cloned.Password = sessions.EncryptedString(password)
	}
	if strings.TrimSpace(keyPassphrase) != "" {
		cloned.KeyPassphrase = sessions.EncryptedString(keyPassphrase)
	}
	cloned.Options = cloneProfileOptionsWithoutCredentialSecrets(cloned.Options)
	return cloned, nil
}

func (s *Service) scrubProfilesForShell(profiles []sessions.Profile) []sessions.Profile {
	scrubbed := make([]sessions.Profile, 0, len(profiles))
	for _, profile := range profiles {
		clone := cloneSessionProfile(profile)
		clone.Password = ""
		clone.KeyPassphrase = ""
		clone.HasPassword = s.store.SecretExists(securestorage.SessionPasswordKey(clone.ID))
		clone.HasKeyPassphrase = s.store.SecretExists(securestorage.SessionKeyPassphraseKey(clone.ID))
		scrubbed = append(scrubbed, clone)
	}
	return scrubbed
}
func profileCredential(profile sessions.Profile) string {
	if strings.EqualFold(strings.TrimSpace(profile.Options["auth_method"]), "key") {
		return string(profile.KeyPassphrase)
	}
	return string(profile.Password)
}

func cloneProfileOptionsWithoutCredentialSecrets(options map[string]string) map[string]string {
	if options == nil {
		return nil
	}
	cloned := make(map[string]string, len(options))
	for key, value := range options {
		if key == "ssh_private_key_passphrase" {
			continue
		}
		cloned[key] = value
	}
	return cloned
}

func cloneSessionProfile(profile sessions.Profile) sessions.Profile {
	cloned := profile
	cloned.Tags = append([]string(nil), profile.Tags...)
	cloned.Options = cloneProfileOptionsWithoutCredentialSecrets(profile.Options)
	return cloned
}

func (s *Service) executeSessionCommand(sessionID, command string) error {
	sessionID = strings.TrimSpace(sessionID)
	command = strings.TrimSpace(command)
	if sessionID == "" {
		return fmt.Errorf("command request session id is required")
	}
	if command == "" {
		return fmt.Errorf("command cannot be empty")
	}
	tab, ok := s.runtimeTab(sessionID)
	if !ok {
		return fmt.Errorf("active session %q not found", sessionID)
	}
	if tab.ProtocolID != "ssh" {
		return fmt.Errorf("commands can only run in ssh sessions")
	}
	if s.sshManager == nil {
		return fmt.Errorf("ssh manager is not configured")
	}
	return s.sshManager.SendInput(sessionID, command+"\n")
}
