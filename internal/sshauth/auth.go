package sshauth

import (
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"

	xssh "golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
)

func BuildAuthMethods(password string, options map[string]string) ([]xssh.AuthMethod, error) {
	authMethods := []xssh.AuthMethod{}
	authMethod := strings.ToLower(strings.TrimSpace(optionValue(options, "auth_method")))
	switch authMethod {
	case "key":
		privateKeyAuth, err := authMethodFromPrivateKey(options)
		if err != nil {
			return nil, err
		}
		authMethods = append(authMethods, privateKeyAuth)
	default:
		if strings.TrimSpace(password) != "" {
			authMethods = append(authMethods, xssh.Password(password))
		}
		if shouldUseAgent(options) {
			agentAuth, err := authMethodFromAgent(options)
			if err != nil {
				return nil, err
			}
			authMethods = append(authMethods, agentAuth)
		}
	}
	if len(authMethods) == 0 {
		return nil, fmt.Errorf("no ssh auth method configured; provide a password or configure an ssh key")
	}
	return authMethods, nil
}

func shouldUseAgent(options map[string]string) bool {
	if options == nil {
		return false
	}
	value := strings.TrimSpace(options["use_ssh_agent"])
	if value == "" {
		value = strings.TrimSpace(options["ssh_agent_socket"])
		return value != ""
	}
	return strings.EqualFold(value, "1") || strings.EqualFold(value, "true") || strings.EqualFold(value, "yes")
}

func authMethodFromAgent(options map[string]string) (xssh.AuthMethod, error) {
	socket := strings.TrimSpace(os.Getenv("SSH_AUTH_SOCK"))
	if options != nil {
		override := strings.TrimSpace(options["ssh_agent_socket"])
		if override != "" && !strings.EqualFold(override, "none") {
			socket = override
		}
	}
	if socket == "" {
		return nil, fmt.Errorf("ssh agent requested but SSH_AUTH_SOCK is not set")
	}
	conn, err := net.Dial("unix", socket)
	if err != nil {
		return nil, fmt.Errorf("connect ssh agent: %w", err)
	}
	agentClient := agent.NewClient(conn)
	return xssh.PublicKeysCallback(agentClient.Signers), nil
}

func authMethodFromPrivateKey(options map[string]string) (xssh.AuthMethod, error) {
	path := strings.TrimSpace(optionValue(options, "ssh_private_key_path"))
	if path == "" {
		return nil, fmt.Errorf("ssh key authentication selected but key path is empty")
	}
	if strings.HasPrefix(path, "~/") {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("resolve user home dir: %w", err)
		}
		path = filepath.Join(homeDir, strings.TrimPrefix(path, "~/"))
	}
	privateKey, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read ssh private key %q: %w", path, err)
	}
	signer, err := xssh.ParsePrivateKey(privateKey)
	if err != nil {
		return nil, fmt.Errorf("parse ssh private key %q: %w", path, err)
	}
	return xssh.PublicKeys(signer), nil
}

func optionValue(options map[string]string, key string) string {
	if options == nil {
		return ""
	}
	return options[key]
}
