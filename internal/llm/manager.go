package llm

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

const (
	defaultModelURL  = "https://huggingface.co/bartowski/Qwen_Qwen3-8B-GGUF/resolve/main/Qwen_Qwen3-8B-Q4_K_M.gguf?download=true"
	defaultModelDir  = "models"
	defaultModelFile = "Qwen_Qwen3-8B-Q4_K_M.gguf"
	defaultHost      = "127.0.0.1"
	defaultPort      = "8012"
)

type Manager struct {
	mu         sync.Mutex
	httpClient *http.Client
	modelURL   string
	modelPath  string
	llamaBin   string
	host       string
	port       string
	serverCmd  *exec.Cmd
}

func NewManager() (*Manager, error) {
	configDir, err := os.UserConfigDir()
	if err != nil {
		return nil, fmt.Errorf("resolve user config dir: %w", err)
	}

	modelPath := filepath.Join(configDir, "opsy", defaultModelDir, defaultModelFile)
	llamaBin := strings.TrimSpace(os.Getenv("OPSY_LLAMA_CPP_BIN"))
	if llamaBin == "" {
		llamaBin = "llama-server"
	}

	return &Manager{
		httpClient: &http.Client{},
		modelURL:   defaultModelURL,
		modelPath:  modelPath,
		llamaBin:   llamaBin,
		host:       defaultHost,
		port:       defaultPort,
	}, nil
}

func (m *Manager) ModelPath() string {
	return m.modelPath
}

func (m *Manager) DownloadQwen3Model(ctx context.Context) (string, error) {
	if info, err := os.Stat(m.modelPath); err == nil && info.Size() > 0 {
		return m.modelPath, nil
	}

	if err := os.MkdirAll(filepath.Dir(m.modelPath), 0o755); err != nil {
		return "", fmt.Errorf("create model directory: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, m.modelURL, nil)
	if err != nil {
		return "", fmt.Errorf("build model download request: %w", err)
	}

	resp, err := m.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("download qwen3 8b model: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("download qwen3 8b model: unexpected status %s", resp.Status)
	}

	tmpPath := m.modelPath + ".part"
	file, err := os.Create(tmpPath)
	if err != nil {
		return "", fmt.Errorf("create temporary model file: %w", err)
	}

	_, copyErr := io.Copy(file, resp.Body)
	closeErr := file.Close()
	if copyErr != nil {
		_ = os.Remove(tmpPath)
		return "", fmt.Errorf("write qwen3 8b model: %w", copyErr)
	}
	if closeErr != nil {
		_ = os.Remove(tmpPath)
		return "", fmt.Errorf("finalize qwen3 8b model: %w", closeErr)
	}

	if err := os.Rename(tmpPath, m.modelPath); err != nil {
		_ = os.Remove(tmpPath)
		return "", fmt.Errorf("move qwen3 8b model into place: %w", err)
	}

	return m.modelPath, nil
}

func (m *Manager) StartLocalServer(ctx context.Context, modelPath string) (string, string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.serverCmd != nil && m.serverCmd.Process != nil && m.serverCmd.ProcessState == nil {
		return m.endpoint(), m.commandLine(modelPath), nil
	}

	if modelPath == "" {
		return "", "", errors.New("download the qwen3 8b model before starting llama.cpp")
	}
	if _, err := os.Stat(modelPath); err != nil {
		return "", "", fmt.Errorf("open local model: %w", err)
	}

	llamaBinary, err := exec.LookPath(m.llamaBin)
	if err != nil {
		return "", "", fmt.Errorf("find llama.cpp server binary %q: %w", m.llamaBin, err)
	}

	cmd := exec.CommandContext(ctx, llamaBinary, "-m", modelPath, "--host", m.host, "--port", m.port)
	var stderr bytes.Buffer
	cmd.Stdout = io.Discard
	cmd.Stderr = &stderr
	if err := cmd.Start(); err != nil {
		return "", "", fmt.Errorf("start llama.cpp server: %w", err)
	}

	m.serverCmd = cmd
	if err := m.waitForPort(); err != nil {
		_ = cmd.Process.Kill()
		m.serverCmd = nil
		if logOutput := strings.TrimSpace(stderr.String()); logOutput != "" {
			return "", "", fmt.Errorf("llama.cpp server did not become ready: %s", logOutput)
		}
		return "", "", err
	}

	go func(command *exec.Cmd) {
		_ = command.Wait()

		m.mu.Lock()
		defer m.mu.Unlock()
		if m.serverCmd == command {
			m.serverCmd = nil
		}
	}(cmd)

	return m.endpoint(), m.commandLine(modelPath), nil
}

func (m *Manager) endpoint() string {
	return fmt.Sprintf("http://%s:%s/v1", m.host, m.port)
}

func (m *Manager) commandLine(modelPath string) string {
	return fmt.Sprintf("%s -m %s --host %s --port %s", m.llamaBin, modelPath, m.host, m.port)
}

func (m *Manager) waitForPort() error {
	address := net.JoinHostPort(m.host, m.port)
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		conn, err := net.DialTimeout("tcp", address, 250*time.Millisecond)
		if err == nil {
			_ = conn.Close()
			return nil
		}
		time.Sleep(200 * time.Millisecond)
	}

	return fmt.Errorf("llama.cpp server did not become ready on %s", address)
}
