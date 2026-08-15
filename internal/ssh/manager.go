package sshmanager

import (
	"context"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"

	xssh "golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/knownhosts"
)

type Manager struct {
	mu          sync.RWMutex
	connections map[string]*connection
	handlers    map[string]func(string)
}

type connection struct {
	client  *xssh.Client
	session *xssh.Session
	stdin   io.WriteCloser
	handler func(string)
}

func NewManager() *Manager {
	return &Manager{connections: map[string]*connection{}, handlers: map[string]func(string){}}
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
		return fmt.Errorf("dial ssh server: %w", err)
	}
	conn, chans, reqs, err := xssh.NewClientConn(netConn, address, config)
	if err != nil {
		_ = netConn.Close()
		return fmt.Errorf("establish ssh session: %w", err)
	}
	client := xssh.NewClient(conn, chans, reqs)
	session, err := client.NewSession()
	if err != nil {
		_ = client.Close()
		return fmt.Errorf("create ssh session: %w", err)
	}
	stdin, err := session.StdinPipe()
	if err != nil {
		_ = session.Close()
		_ = client.Close()
		return fmt.Errorf("open ssh stdin: %w", err)
	}
	stdout, err := session.StdoutPipe()
	if err != nil {
		_ = session.Close()
		_ = client.Close()
		return fmt.Errorf("open ssh stdout: %w", err)
	}
	stderr, err := session.StderrPipe()
	if err != nil {
		_ = session.Close()
		_ = client.Close()
		return fmt.Errorf("open ssh stderr: %w", err)
	}

	modes := xssh.TerminalModes{xssh.ECHO: 1, xssh.TTY_OP_ISPEED: 14400, xssh.TTY_OP_OSPEED: 14400}
	if err := session.RequestPty("xterm-256color", 24, 80, modes); err != nil {
		_ = session.Close()
		_ = client.Close()
		return fmt.Errorf("request ssh pty: %w", err)
	}
	if err := session.Shell(); err != nil {
		_ = session.Close()
		_ = client.Close()
		return fmt.Errorf("start ssh shell: %w", err)
	}

	m.mu.Lock()
	handler := m.handlers[tabID]
	connState := &connection{client: client, session: session, stdin: stdin, handler: handler}
	m.connections[tabID] = connState
	m.mu.Unlock()

	go m.streamOutput(tabID, stdout)
	go m.streamOutput(tabID, stderr)
	go func() {
		_ = session.Wait()
		_ = m.Disconnect(tabID)
	}()

	return nil
}

func (m *Manager) SendInput(tabID, data string) error {
	m.mu.RLock()
	conn := m.connections[tabID]
	m.mu.RUnlock()
	if conn == nil {
		return fmt.Errorf("ssh tab %q is not connected", tabID)
	}
	if _, err := io.WriteString(conn.stdin, data); err != nil {
		return fmt.Errorf("write ssh input: %w", err)
	}
	return nil
}

func (m *Manager) ResizeTerminal(tabID string, cols, rows int) error {
	m.mu.RLock()
	conn := m.connections[tabID]
	m.mu.RUnlock()
	if conn == nil {
		return fmt.Errorf("ssh tab %q is not connected", tabID)
	}
	if err := conn.session.WindowChange(rows, cols); err != nil {
		return fmt.Errorf("resize ssh terminal: %w", err)
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
	if conn.stdin != nil {
		_ = conn.stdin.Close()
	}
	if conn.session != nil {
		_ = conn.session.Close()
	}
	if conn.client != nil {
		_ = conn.client.Close()
	}
	return nil
}

func (m *Manager) SetOutputHandler(tabID string, fn func(data string)) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.handlers[tabID] = fn
	if conn := m.connections[tabID]; conn != nil {
		conn.handler = fn
	}
}

func (m *Manager) GetCurrentDir(tabID string) (string, error) {
	m.mu.RLock()
	conn := m.connections[tabID]
	m.mu.RUnlock()
	if conn == nil {
		return "", fmt.Errorf("ssh tab %q is not connected", tabID)
	}
	session, err := conn.client.NewSession()
	if err != nil {
		return "", fmt.Errorf("create ssh pwd session: %w", err)
	}
	defer session.Close()
	output, err := session.CombinedOutput("pwd")
	if err != nil {
		return "", fmt.Errorf("run pwd over ssh: %w", err)
	}
	return strings.TrimSpace(string(output)), nil
}

func (m *Manager) streamOutput(tabID string, reader io.Reader) {
	buffer := make([]byte, 4096)
	for {
		n, err := reader.Read(buffer)
		if n > 0 {
			m.mu.RLock()
			handler := m.handlers[tabID]
			m.mu.RUnlock()
			if handler != nil {
				handler(string(buffer[:n]))
			}
		}
		if err != nil {
			return
		}
	}
}

func hostKeyCallback() (xssh.HostKeyCallback, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("resolve user home dir: %w", err)
	}
	return knownhosts.New(filepath.Join(homeDir, ".ssh", "known_hosts"))
}
