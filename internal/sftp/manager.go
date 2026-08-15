package sftpmanager

import (
	"context"
	"fmt"
	"net"
	"os"
	"path"
	"path/filepath"
	"sync"
	"time"

	sftpdomain "opsy/internal/domain/sftp"

	pkgsftp "github.com/pkg/sftp"
	xssh "golang.org/x/crypto/ssh"
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

func (m *Manager) Connect(ctx context.Context, tabID, host string, port int, user, password string) error {
	if port <= 0 {
		port = 22
	}
	_ = m.Disconnect(tabID)

	hostKeyCallback, err := hostKeyCallback()
	if err != nil {
		return err
	}
	config := &xssh.ClientConfig{
		User:            user,
		Auth:            []xssh.AuthMethod{xssh.Password(password)},
		HostKeyCallback: hostKeyCallback,
	}
	address := net.JoinHostPort(host, fmt.Sprintf("%d", port))
	dialer := &net.Dialer{}
	netConn, err := dialer.DialContext(ctx, "tcp", address)
	if err != nil {
		return fmt.Errorf("dial sftp server: %w", err)
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
