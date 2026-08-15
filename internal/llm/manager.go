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
	defaultModelURL    = "https://huggingface.co/bartowski/Qwen_Qwen3-8B-GGUF/resolve/main/Qwen_Qwen3-8B-Q4_K_M.gguf?download=true"
	defaultOpsyDir     = ".opsy"
	defaultModelDir    = "models"
	defaultModelFile   = "Qwen_Qwen3-8B-Q4_K_M.gguf"
	defaultHost        = "127.0.0.1"
	defaultPort        = "8012"
	serverReadyTimeout = 30 * time.Second
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

func OpsyDir() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve user home dir: %w", err)
	}
	return filepath.Join(homeDir, defaultOpsyDir), nil
}

func NewManager() (*Manager, error) {
	opsyDir, err := OpsyDir()
	if err != nil {
		return nil, err
	}

	modelPath := filepath.Join(opsyDir, defaultModelDir, defaultModelFile)
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
	return m.DownloadQwen3ModelWithProgress(ctx, nil)
}

func (m *Manager) DownloadQwen3ModelWithProgress(ctx context.Context, progressFn func(downloaded, total int64)) (string, error) {
	if info, err := os.Stat(m.modelPath); err == nil && info.Size() > 0 {
		if progressFn != nil {
			progressFn(info.Size(), info.Size())
		}
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

	writer := io.Writer(file)
	if progressFn != nil {
		writer = io.MultiWriter(file, &progressWriter{total: resp.ContentLength, fn: progressFn})
	}

	_, copyErr := io.Copy(writer, resp.Body)
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

	if progressFn != nil {
		if info, err := os.Stat(m.modelPath); err == nil {
			progressFn(info.Size(), maxInt64(resp.ContentLength, info.Size()))
		}
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
	deadline := time.Now().Add(serverReadyTimeout)
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

type progressWriter struct {
	downloaded int64
	total      int64
	fn         func(downloaded, total int64)
}

func (p *progressWriter) Write(data []byte) (int, error) {
	n := len(data)
	p.downloaded += int64(n)
	p.fn(p.downloaded, p.total)
	return n, nil
}

func maxInt64(left, right int64) int64 {
	if left > right {
		return left
	}
	return right
}
