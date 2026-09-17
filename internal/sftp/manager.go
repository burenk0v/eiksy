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

	sftpdomain "eiksy/internal/domain/sftp"
	"eiksy/internal/sshauth"

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

const (
	maxSFTPReadSize = 16 << 20
	maxSFTPTransferSize = 256 << 20
)

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
		result = append(result, sftpdomain.FileEntry{Name: entry.Name(), Path: path.Join(targetPath, entry.Name()), IsDir: entry.IsDir(), Size: entry.Size(), ModTime: entry.ModTime().UTC().Format(time.RFC3339), Mode: entry.Mode().String()})
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
	data, err := io.ReadAll(io.LimitReader(file, maxSFTPReadSize+1))
	if err != nil {
		return "", fmt.Errorf("read sftp file %q: %w", targetPath, err)
	}
	if len(data) > maxSFTPReadSize {
		return "", fmt.Errorf("read sftp file %q: file exceeds %d byte limit", targetPath, maxSFTPReadSize)
	}
	return string(data), nil
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

func (m *Manager) UploadFile(tabID, localPath, remotePath string) error {
	m.mu.RLock()
	conn := m.connections[tabID]
	m.mu.RUnlock()
	if conn == nil {
		return fmt.Errorf("sftp tab %q is not connected", tabID)
	}
	localPath, err := validateLocalUploadPath(localPath)
	if err != nil {
		return err
	}
	source, err := os.Open(localPath)
	if err != nil {
		return fmt.Errorf("open local file %q: %w", localPath, err)
	}
	defer source.Close()
	openedInfo, err := source.Stat()
	if err != nil {
		return fmt.Errorf("inspect opened local file %q: %w", localPath, err)
	}
	pathInfo, err := os.Lstat(localPath)
	if err != nil || pathInfo.Mode()&os.ModeSymlink != 0 || !os.SameFile(openedInfo, pathInfo) {
		return fmt.Errorf("local upload path %q changed while opening", localPath)
	}
	target, err := conn.sftpClient.OpenFile(remotePath, os.O_WRONLY|os.O_TRUNC|os.O_CREATE)
	if err != nil {
		return fmt.Errorf("open remote file %q for write: %w", remotePath, err)
	}
	defer target.Close()
	limited := io.LimitReader(source, maxSFTPTransferSize+1)
	written, err := io.Copy(target, limited)
	if err != nil {
		return fmt.Errorf("upload file %q to %q: %w", localPath, remotePath, err)
	}
	if written > maxSFTPTransferSize {
		return fmt.Errorf("upload file %q exceeds %d byte limit", localPath, maxSFTPTransferSize)
	}
	return nil
}

func (m *Manager) DownloadFile(tabID, remotePath, localPath string) error {
	m.mu.RLock()
	conn := m.connections[tabID]
	m.mu.RUnlock()
	if conn == nil {
		return fmt.Errorf("sftp tab %q is not connected", tabID)
	}
	localPath, err := validateLocalDownloadPath(localPath)
	if err != nil {
		return err
	}
	source, err := conn.sftpClient.Open(remotePath)
	if err != nil {
		return fmt.Errorf("open remote file %q: %w", remotePath, err)
	}
	defer source.Close()
	target, err := os.CreateTemp(filepath.Dir(localPath), ".eiksy-download-*")
	if err != nil {
		return fmt.Errorf("create temporary download file for %q: %w", localPath, err)
	}
	temporaryPath := target.Name()
	defer os.Remove(temporaryPath)
	if err := target.Chmod(0o600); err != nil {
		_ = target.Close()
		return fmt.Errorf("set temporary download permissions: %w", err)
	}
	limited := io.LimitReader(source, maxSFTPTransferSize+1)
	written, err := io.Copy(target, limited)
	if err != nil {
		_ = target.Close()
		return fmt.Errorf("download file %q to %q: %w", remotePath, localPath, err)
	}
	if written > maxSFTPTransferSize {
		_ = target.Close()
		return fmt.Errorf("download file %q exceeds %d byte limit", remotePath, maxSFTPTransferSize)
	}
	if err := target.Close(); err != nil {
		return fmt.Errorf("close temporary download file: %w", err)
	}
	if err := os.Rename(temporaryPath, localPath); err != nil {
		return fmt.Errorf("publish download to %q: %w", localPath, err)
	}
	return nil
}

func validateLocalUploadPath(localPath string) (string, error) {
	localPath = strings.TrimSpace(localPath)
	if localPath == "" {
		return "", fmt.Errorf("local file path is required")
	}
	if !filepath.IsAbs(localPath) {
		return "", fmt.Errorf("local file path must be absolute")
	}
	info, err := os.Lstat(localPath)
	if err != nil {
		return "", fmt.Errorf("inspect local file %q: %w", localPath, err)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return "", fmt.Errorf("local upload path %q must not be a symlink", localPath)
	}
	if !info.Mode().IsRegular() {
		return "", fmt.Errorf("local upload path %q is not a regular file", localPath)
	}
	return filepath.Clean(localPath), nil
}

func validateLocalDownloadPath(localPath string) (string, error) {
	localPath = strings.TrimSpace(localPath)
	if localPath == "" {
		return "", fmt.Errorf("local file path is required")
	}
	if !filepath.IsAbs(localPath) {
		return "", fmt.Errorf("local file path must be absolute")
	}
	localPath = filepath.Clean(localPath)
	parent := filepath.Dir(localPath)
	info, err := os.Stat(parent)
	if err != nil {
		return "", fmt.Errorf("inspect local download directory %q: %w", parent, err)
	}
	if !info.IsDir() {
		return "", fmt.Errorf("local download parent %q is not a directory", parent)
	}
	if info, err := os.Lstat(localPath); err == nil {
		if info.Mode()&os.ModeSymlink != 0 {
			return "", fmt.Errorf("local download path %q must not be a symlink", localPath)
		}
		if info.IsDir() {
			return "", fmt.Errorf("local download path %q is a directory", localPath)
		}
	} else if !os.IsNotExist(err) {
		return "", fmt.Errorf("inspect local download path %q: %w", localPath, err)
	}
	return localPath, nil
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

func knownHostsPath() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve user home dir: %w", err)
	}
	return filepath.Join(homeDir, ".ssh", "known_hosts"), nil
}

func hostKeyCallback() (xssh.HostKeyCallback, error) {
	path, err := knownHostsPath()
	if err != nil {
		return nil, err
	}
	if _, err := os.Stat(path); err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("known_hosts file not found at %q; add the server host key before using SFTP", path)
		}
		return nil, fmt.Errorf("stat known_hosts %q: %w", path, err)
	}
	callback, err := knownhosts.New(path)
	if err != nil {
		return nil, fmt.Errorf("load known_hosts: %w", err)
	}
	return callback, nil
}

func buildClientConfig(user, password string, options map[string]string, hostKeyCallback xssh.HostKeyCallback) (*xssh.ClientConfig, error) {
	authMethods, err := sshauth.BuildAuthMethods(password, options)
	if err != nil {
		return nil, err
	}
	return &xssh.ClientConfig{User: user, Auth: authMethods, HostKeyCallback: hostKeyCallback}, nil
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
