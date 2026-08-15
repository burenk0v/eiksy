package sftpmanager

import (
	"context"
	"fmt"
	"io"
	"net"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	sftpdomain "opsy/internal/domain/sftp"

	pkgsftp "github.com/pkg/sftp"
	xssh "golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
	"golang.org/x/crypto/ssh/knownhosts"
)

type Manager struct {
	mu          sync.RWMutex
	connections map[string]*connection
}

type connection struct {
	sshClient  *xssh.Client
	sftpClient *pkgsftp.Client
}

func NewManager() *Manager {
	return &Manager{connections: map[string]*connection{}}
}

func (m *Manager) Connect(ctx context.Context, tabID, host string, port int, user, password string, options map[string]string) error {
	if port <= 0 {
		port = 22
	}
	_ = m.Disconnect(tabID)

	hostKey, err := hostKeyCallback()
	if err != nil {
		return err
	}
	config, err := buildClientConfig(user, password, options, hostKey)
	if err != nil {
		return err
	}
	address := net.JoinHostPort(host, strconv.Itoa(port))
	netConn, err := dialTarget(ctx, address, config, options)
	if err != nil {
		return err
	}

	conn, chans, reqs, err := xssh.NewClientConn(netConn, address, config)
	if err != nil {
		_ = netConn.Close()
		return fmt.Errorf("establish sftp ssh session: %w", err)
	}
	sshClient := xssh.NewClient(conn, chans, reqs)
	sftpClient, err := pkgsftp.NewClient(sshClient)
	if err != nil {
		_ = sshClient.Close()
		return fmt.Errorf("create sftp client: %w", err)
	}

	m.mu.Lock()
	m.connections[tabID] = &connection{sshClient: sshClient, sftpClient: sftpClient}
	m.mu.Unlock()
	return nil
}

func (m *Manager) ListDir(tabID, targetPath string) ([]sftpdomain.FileEntry, error) {
	m.mu.RLock()
	conn := m.connections[tabID]
	m.mu.RUnlock()
	if conn == nil {
		return nil, fmt.Errorf("sftp tab %q is not connected", tabID)
	}
	if targetPath == "" {
		targetPath = "."
	}
	entries, err := conn.sftpClient.ReadDir(targetPath)
	if err != nil {
		return nil, fmt.Errorf("list sftp directory %q: %w", targetPath, err)
	}
	result := make([]sftpdomain.FileEntry, 0, len(entries))
	for _, entry := range entries {
		result = append(result, sftpdomain.FileEntry{
			Name:    entry.Name(),
			Path:    path.Join(targetPath, entry.Name()),
			IsDir:   entry.IsDir(),
			Size:    entry.Size(),
			ModTime: entry.ModTime().UTC().Format(time.RFC3339),
			Mode:    entry.Mode().String(),
		})
	}
	return result, nil
}

func (m *Manager) ReadFile(tabID, targetPath string) (string, error) {
	m.mu.RLock()
	conn := m.connections[tabID]
	m.mu.RUnlock()
	if conn == nil {
		return "", fmt.Errorf("sftp tab %q is not connected", tabID)
	}
	file, err := conn.sftpClient.Open(targetPath)
	if err != nil {
		return "", fmt.Errorf("read sftp file %q: %w", targetPath, err)
	}
	defer file.Close()
	bytes, err := io.ReadAll(file)
	if err != nil {
		return "", fmt.Errorf("read sftp file %q: %w", targetPath, err)
	}
	return string(bytes), nil
}

func (m *Manager) WriteFile(tabID, targetPath, content string) error {
	m.mu.RLock()
	conn := m.connections[tabID]
	m.mu.RUnlock()
	if conn == nil {
		return fmt.Errorf("sftp tab %q is not connected", tabID)
	}
	file, err := conn.sftpClient.OpenFile(targetPath, os.O_WRONLY|os.O_TRUNC|os.O_CREATE)
	if err != nil {
		return fmt.Errorf("open sftp file %q for write: %w", targetPath, err)
	}
	defer file.Close()
	if _, err := file.Write([]byte(content)); err != nil {
		return fmt.Errorf("write sftp file %q: %w", targetPath, err)
	}
	return nil
}

func (m *Manager) Disconnect(tabID string) error {
	m.mu.Lock()
	conn := m.connections[tabID]
	delete(m.connections, tabID)
	m.mu.Unlock()
	if conn == nil {
		return nil
	}
	if conn.sftpClient != nil {
		_ = conn.sftpClient.Close()
	}
	if conn.sshClient != nil {
		_ = conn.sshClient.Close()
	}
	return nil
}

func (m *Manager) Connected(tabID string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	_, ok := m.connections[tabID]
	return ok
}

func hostKeyCallback() (xssh.HostKeyCallback, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("resolve user home dir: %w", err)
	}
	knownHostsPath := filepath.Join(homeDir, ".ssh", "known_hosts")
	if _, err := os.Stat(knownHostsPath); os.IsNotExist(err) {
		return xssh.InsecureIgnoreHostKey(), nil //nolint:gosec
	}
	return knownhosts.New(knownHostsPath)
}

func buildClientConfig(user, password string, options map[string]string, hostKeyCallback xssh.HostKeyCallback) (*xssh.ClientConfig, error) {
	authMethods := []xssh.AuthMethod{}
	if strings.TrimSpace(password) != "" {
		authMethods = append(authMethods, xssh.Password(password))
	}
	if shouldUseAgent(options) {
		authMethod, err := authMethodFromAgent(options)
		if err != nil {
			return nil, err
		}
		authMethods = append(authMethods, authMethod)
	}
	if len(authMethods) == 0 {
		return nil, fmt.Errorf("no ssh auth method configured; provide a password or enable ssh agent")
	}
	return &xssh.ClientConfig{User: user, Auth: authMethods, HostKeyCallback: hostKeyCallback}, nil
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

func dialTarget(ctx context.Context, address string, config *xssh.ClientConfig, options map[string]string) (net.Conn, error) {
	proxyJump := strings.TrimSpace(optionValue(options, "proxy_jump"))
	if proxyJump == "" {
		dialer := &net.Dialer{}
		netConn, err := dialer.DialContext(ctx, "tcp", address)
		if err != nil {
			return nil, fmt.Errorf("dial sftp server: %w", err)
		}
		return netConn, nil
	}
	jumpAddress, jumpUser := parseProxyJump(proxyJump, config.User)
	jumpConfig := cloneClientConfig(config)
	jumpConfig.User = jumpUser
	jumpClient, err := xssh.Dial("tcp", jumpAddress, jumpConfig)
	if err != nil {
		return nil, fmt.Errorf("dial proxy jump %s: %w", jumpAddress, err)
	}
	netConn, err := jumpClient.Dial("tcp", address)
	if err != nil {
		_ = jumpClient.Close()
		return nil, fmt.Errorf("dial target via proxy jump: %w", err)
	}
	return &proxyConn{Conn: netConn, jumpClient: jumpClient}, nil
}

func parseProxyJump(value, defaultUser string) (string, string) {
	value = strings.TrimSpace(value)
	value = strings.Split(value, ",")[0]
	user := defaultUser
	hostPort := value
	if at := strings.Index(value, "@"); at > 0 {
		user = value[:at]
		hostPort = value[at+1:]
	}
	host, port, err := net.SplitHostPort(hostPort)
	if err != nil || host == "" {
		host = hostPort
		port = "22"
	}
	return net.JoinHostPort(host, port), user
}

func cloneClientConfig(config *xssh.ClientConfig) *xssh.ClientConfig {
	clone := *config
	clone.Auth = append([]xssh.AuthMethod(nil), config.Auth...)
	return &clone
}

func optionValue(options map[string]string, key string) string {
	if options == nil {
		return ""
	}
	return options[key]
}

type proxyConn struct {
	net.Conn
	jumpClient *xssh.Client
}

func (c *proxyConn) Close() error {
	_ = c.Conn.Close()
	if c.jumpClient != nil {
		return c.jumpClient.Close()
	}
	return nil
}
