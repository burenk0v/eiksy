package sshmanager

import (
	"context"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"

	"opsy/internal/sshauth"

	xssh "golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/knownhosts"
)

type Manager struct {
	mu          sync.RWMutex
	connections map[string]*connection
	handlers    map[string]func(string)
}

type connection struct {
	client         *xssh.Client
	session        *xssh.Session
	stdin          io.WriteCloser
	handler        func(string)
	localListeners []net.Listener
}

func NewManager() *Manager {
	return &Manager{connections: map[string]*connection{}, handlers: map[string]func(string){}}
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

	listeners, err := startLocalForwards(client, options)
	if err != nil {
		_ = session.Close()
		_ = client.Close()
		return err
	}

	m.mu.Lock()
	handler := m.handlers[tabID]
	connState := &connection{client: client, session: session, stdin: stdin, handler: handler, localListeners: listeners}
	m.connections[tabID] = connState
	m.mu.Unlock()

	go m.streamOutput(tabID, stdout)
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
	for _, listener := range conn.localListeners {
		_ = listener.Close()
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
	knownHostsPath := filepath.Join(homeDir, ".ssh", "known_hosts")
	if _, err := os.Stat(knownHostsPath); os.IsNotExist(err) {
		return xssh.InsecureIgnoreHostKey(), nil //nolint:gosec
	}
	return knownhosts.New(knownHostsPath)
}

func buildClientConfig(user, password string, options map[string]string, hostKeyCallback xssh.HostKeyCallback) (*xssh.ClientConfig, error) {
	authMethods, err := sshauth.BuildAuthMethods(password, options)
	if err != nil {
		return nil, err
	}
	return &xssh.ClientConfig{
		User:            user,
		Auth:            authMethods,
		HostKeyCallback: hostKeyCallback,
	}, nil
}

func dialTarget(ctx context.Context, address string, config *xssh.ClientConfig, options map[string]string) (net.Conn, error) {
	proxyJump := strings.TrimSpace(optionValue(options, "proxy_jump"))
	if proxyJump == "" {
		dialer := &net.Dialer{}
		netConn, err := dialer.DialContext(ctx, "tcp", address)
		if err != nil {
			return nil, fmt.Errorf("dial ssh server: %w", err)
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

func startLocalForwards(client *xssh.Client, options map[string]string) ([]net.Listener, error) {
	if options == nil {
		return nil, nil
	}
	raw := strings.TrimSpace(options["local_forwards"])
	if raw == "" {
		return nil, nil
	}
	specs := strings.Split(raw, ",")
	listeners := make([]net.Listener, 0, len(specs))
	for _, spec := range specs {
		bindAddr, remoteAddr, err := parseLocalForwardSpec(spec)
		if err != nil {
			for _, listener := range listeners {
				_ = listener.Close()
			}
			return nil, err
		}
		listener, err := net.Listen("tcp", bindAddr)
		if err != nil {
			for _, current := range listeners {
				_ = current.Close()
			}
			return nil, fmt.Errorf("open local tunnel %s: %w", bindAddr, err)
		}
		listeners = append(listeners, listener)
		go handleTunnelListener(listener, client, remoteAddr)
	}
	return listeners, nil
}

func handleTunnelListener(listener net.Listener, client *xssh.Client, remoteAddr string) {
	for {
		localConn, err := listener.Accept()
		if err != nil {
			return
		}
		go func() {
			defer localConn.Close()
			remoteConn, err := client.Dial("tcp", remoteAddr)
			if err != nil {
				return
			}
			defer remoteConn.Close()
			go io.Copy(remoteConn, localConn)
			_, _ = io.Copy(localConn, remoteConn)
		}()
	}
}

func parseLocalForwardSpec(spec string) (string, string, error) {
	spec = strings.TrimSpace(spec)
	parts := strings.Split(spec, ":")
	if len(parts) != 3 {
		return "", "", fmt.Errorf("invalid local forward %q, expected localPort:remoteHost:remotePort", spec)
	}
	localPort := strings.TrimSpace(parts[0])
	remoteHost := strings.TrimSpace(parts[1])
	remotePort := strings.TrimSpace(parts[2])
	if localPort == "" || remoteHost == "" || remotePort == "" {
		return "", "", fmt.Errorf("invalid local forward %q", spec)
	}
	return net.JoinHostPort("127.0.0.1", localPort), net.JoinHostPort(remoteHost, remotePort), nil
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
