package app

import (
	"fmt"
	"strconv"
	"strings"

	"opsy/internal/domain/sessions"
)

func (s *Service) ImportSSHConfig(raw string) ([]sessions.Profile, error) {
	mutator, ok := s.store.(sessionProfileMutator)
	if !ok {
		return nil, fmt.Errorf("session profiles are read-only")
	}
	entries := parseSSHConfig(raw)
	if len(entries) == 0 {
		return nil, fmt.Errorf("no importable host entries found")
	}

	imported := make([]sessions.Profile, 0, len(entries))
	for _, entry := range entries {
		if entry.hostAlias == "" {
			continue
		}
		profile := sessions.Profile{
			ID:         strings.ToLower(strings.ReplaceAll(entry.hostAlias, " ", "-")),
			Name:       entry.hostAlias,
			Group:      "Imported SSH config",
			ProtocolID: "ssh",
			Host:       defaultString(entry.hostName, entry.hostAlias),
			Port:       entry.port,
			Username:   entry.user,
			Options:    map[string]string{},
		}
		if profile.Port <= 0 {
			profile.Port = 22
		}
		if entry.proxyJump != "" {
			profile.Options["proxy_jump"] = entry.proxyJump
		}
		if entry.identityAgent != "" && !strings.EqualFold(entry.identityAgent, "none") {
			profile.Options["ssh_agent_socket"] = entry.identityAgent
		}
		if entry.identityFile != "" {
			profile.Options["auth_method"] = "key"
			profile.Options["ssh_private_key_path"] = entry.identityFile
		}
		if len(entry.localForwards) > 0 {
			profile.Options["local_forwards"] = strings.Join(entry.localForwards, ",")
		}
		if len(profile.Options) == 0 {
			profile.Options = nil
		}
		profile = normalizeProfile(profile)
		if err := s.validateProfile(profile); err != nil {
			continue
		}
		if err := mutator.UpsertSessionProfile(profile); err != nil {
			return nil, err
		}
		imported = append(imported, profile)
	}
	if len(imported) == 0 {
		return nil, fmt.Errorf("no valid ssh profiles were imported")
	}
	s.EmitLog("info", fmt.Sprintf("Imported %d SSH profile(s)", len(imported)))
	return imported, nil
}

type sshConfigEntry struct {
	hostAlias     string
	hostName      string
	user          string
	port          int
	proxyJump     string
	identityAgent string
	identityFile  string
	localForwards []string
}

func parseSSHConfig(raw string) []sshConfigEntry {
	lines := strings.Split(raw, "\n")
	entries := []sshConfigEntry{}
	current := sshConfigEntry{port: 22}

	flush := func() {
		if current.hostAlias == "" {
			return
		}
		entries = append(entries, current)
		current = sshConfigEntry{port: 22}
	}

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		line = stripInlineComment(line)
		parts := strings.Fields(line)
		if len(parts) < 2 {
			continue
		}
		key := strings.ToLower(parts[0])
		value := strings.TrimSpace(strings.Join(parts[1:], " "))
		switch key {
		case "host":
			flush()
			hostAlias := strings.Fields(value)
			if len(hostAlias) == 0 || strings.Contains(hostAlias[0], "*") || strings.Contains(hostAlias[0], "?") {
				continue
			}
			current.hostAlias = hostAlias[0]
		case "hostname":
			current.hostName = value
		case "user":
			current.user = value
		case "port":
			port, err := strconv.Atoi(value)
			if err == nil && port > 0 {
				current.port = port
			}
		case "proxyjump":
			current.proxyJump = value
		case "identityagent":
			current.identityAgent = value
		case "identityfile":
			current.identityFile = value
		case "localforward":
			current.localForwards = append(current.localForwards, value)
		}
	}
	flush()
	return entries
}

func stripInlineComment(line string) string {
	if index := strings.Index(line, "#"); index >= 0 {
		return strings.TrimSpace(line[:index])
	}
	return line
}

func defaultString(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}
