package main

import (
	"context"
	"log"
	"sync"

	"opsy/internal/app"
	"opsy/internal/domain/sessions"
	"opsy/internal/domain/settings"
	sftpdomain "opsy/internal/domain/sftp"
	vaultdomain "opsy/internal/domain/vault"
	"opsy/internal/llm"
	sftpmanager "opsy/internal/sftp"
	sshmanager "opsy/internal/ssh"
	"opsy/internal/storage/disk"
	"opsy/internal/storage/memory"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// App wires the Wails bridge to backend services.
type App struct {
	ctx            context.Context
	service        *app.Service
	downloadMu     sync.Mutex
	downloadCancel context.CancelFunc
}

// NewApp creates the root application instance.
func NewApp() *App {
	return &App{}
}

// startup is called when the app starts.
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx

	store, storeErr := disk.NewStore()
	if storeErr != nil {
		log.Printf("disk store unavailable, falling back to memory store: %v", storeErr)
		store = nil
	}

	localManager, err := llm.NewManager()
	if err != nil {
		log.Printf("local model manager unavailable: %v", err)
	}

	if store != nil {
		a.service = app.NewService(store, localManager, sshmanager.NewManager(), sftpmanager.NewManager())
	} else {
		a.service = app.NewService(memory.NewStore(), localManager, sshmanager.NewManager(), sftpmanager.NewManager())
	}
	a.service.SetRuntimeContext(ctx, func(eventName string, data ...interface{}) {
		runtime.EventsEmit(ctx, eventName, data...)
	})

	if storeErr != nil {
		a.service.EmitLog("error", "Disk store could not be initialized; running with in-memory defaults.")
	} else {
		a.service.EmitLog("info", "Application started with disk storage.")
	}
}

// GetShellState returns the full backend-owned application state for the UI shell.
func (a *App) GetShellState() app.ShellState {
	return a.currentService().GetShellState()
}

// LaunchSession opens a new runtime tab from a saved session profile.
func (a *App) LaunchSession(profileID string) (app.RuntimeSessionView, error) {
	return a.currentService().LaunchSession(profileID)
}

// CloseSession closes an active runtime tab.
func (a *App) CloseSession(sessionID string) error {
	return a.currentService().CloseSession(sessionID)
}

func (a *App) CreateSessionProfile(input sessions.ProfileInput) error {
	return a.currentService().CreateSessionProfile(input.ToProfile())
}

func (a *App) DeleteSessionProfile(id string) error {
	return a.currentService().DeleteSessionProfile(id)
}

func (a *App) ConnectSSH(tabID, profileID string) error {
	return a.currentService().ConnectSSH(a.ctx, tabID, profileID)
}

func (a *App) SendSSHInput(tabID, data string) error {
	return a.currentService().SendSSHInput(tabID, data)
}

func (a *App) ResizeTerminal(tabID string, cols, rows int) error {
	return a.currentService().ResizeTerminal(tabID, cols, rows)
}

func (a *App) DisconnectSSH(tabID string) error {
	return a.currentService().DisconnectSSH(tabID)
}

func (a *App) ListSFTPFiles(tabID, path string) ([]sftpdomain.FileEntry, error) {
	return a.currentService().ListSFTPFiles(tabID, path)
}

func (a *App) NavigateSFTP(tabID, path string) ([]sftpdomain.FileEntry, error) {
	return a.currentService().NavigateSFTP(tabID, path)
}

func (a *App) ReadSFTPFile(tabID, path string) (string, error) {
	return a.currentService().ReadSFTPFile(tabID, path)
}

func (a *App) SaveSFTPFile(tabID, path, content string) error {
	return a.currentService().SaveSFTPFile(tabID, path, content)
}

func (a *App) SelectUploadFiles() ([]string, error) {
	return runtime.OpenMultipleFilesDialog(a.ctx, runtime.OpenDialogOptions{
		Title: "Select file(s) to upload",
	})
}

func (a *App) SelectDownloadDirectory() (string, error) {
	return runtime.OpenDirectoryDialog(a.ctx, runtime.OpenDialogOptions{
		Title: "Select folder for downloaded files",
	})
}

func (a *App) UploadSFTPFiles(tabID, remoteDir string, localPaths []string) error {
	return a.currentService().UploadSFTPFiles(tabID, remoteDir, localPaths)
}

func (a *App) DownloadSFTPFiles(tabID, localDir string, remotePaths []string) error {
	return a.currentService().DownloadSFTPFiles(tabID, localDir, remotePaths)
}

func (a *App) OpenRDP(tabID, profileID string) error {
	_, err := a.currentService().OpenRDP(tabID, profileID)
	return err
}

func (a *App) ListVaultSecrets(path string) ([]vaultdomain.SecretNode, error) {
	return a.currentService().ListVaultSecrets(path)
}

func (a *App) ImportSSHConfig(raw string) ([]sessions.Profile, error) {
	return a.currentService().ImportSSHConfig(raw)
}

func (a *App) OpenSessionWindow() error {
	return nil
}

func (a *App) UpdateSettings(input settings.AppSettings) error {
	return a.currentService().UpdateSettings(input)
}

func (a *App) SelectAIProvider(providerID string) error {
	return a.currentService().SelectAIProvider(providerID)
}

func (a *App) SaveCloudProvider(model string, endpoint string, token string) error {
	return a.currentService().SaveCloudProvider(model, endpoint, token)
}

func (a *App) ListCloudModels(endpoint string, token string) ([]string, error) {
	return a.currentService().ListCloudModels(endpoint, token)
}

func (a *App) DownloadLocalModel() error {
	return a.currentService().DownloadLocalModel(a.ctx)
}

func (a *App) CancelModelDownload() {
	a.downloadMu.Lock()
	cancel := a.downloadCancel
	a.downloadCancel = nil
	a.downloadMu.Unlock()
	if cancel != nil {
		cancel()
	}
}

func (a *App) DownloadLocalModelWithProgress() error {
	ctx, cancel := context.WithCancel(a.ctx)
	a.downloadMu.Lock()
	a.downloadCancel = cancel
	a.downloadMu.Unlock()
	defer func() {
		a.downloadMu.Lock()
		a.downloadCancel = nil
		a.downloadMu.Unlock()
		cancel()
	}()
	err := a.currentService().DownloadLocalModelWithProgress(ctx, func(downloaded, total int64) {
		percent := 0.0
		if total > 0 {
			percent = float64(downloaded) / float64(total) * 100
		}
		runtime.EventsEmit(a.ctx, "model:progress", map[string]interface{}{
			"downloaded": downloaded,
			"total":      total,
			"percent":    percent,
		})
	})
	if err != nil {
		runtime.EventsEmit(a.ctx, "model:error", map[string]string{"error": err.Error()})
		return err
	}
	return nil
}

func (a *App) StartLocalModel() error {
	return a.currentService().StartLocalModel(a.ctx)
}

func (a *App) SendChatMessage(message string) error {
	return a.currentService().SendChatMessage(a.ctx, message)
}

func (a *App) ClearChat() {
	a.currentService().ClearChat()
}

func (a *App) AcceptSSHHostKey(tabID string) error {
	return a.currentService().AcceptSSHHostKey(tabID)
}

func (a *App) currentService() *app.Service {
	if a.service == nil {
		svc := app.NewService(memory.NewStore(), nil, sshmanager.NewManager(), sftpmanager.NewManager())
		svc.SetRuntimeContext(a.ctx, func(eventName string, data ...interface{}) {
			runtime.EventsEmit(a.ctx, eventName, data...)
		})
		a.service = svc
	}
	return a.service
}
