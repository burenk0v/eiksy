package main

import (
	"context"
	"log"
	"os"
	"path/filepath"
	"strconv"

	"eiksy/internal/app"
	"eiksy/internal/domain/ai"
	"eiksy/internal/domain/sessions"
	"eiksy/internal/domain/settings"
	sftpdomain "eiksy/internal/domain/sftp"
	vaultdomain "eiksy/internal/domain/vault"
	"eiksy/internal/securestorage"
	sftpmanager "eiksy/internal/sftp"
	sshmanager "eiksy/internal/ssh"
	"eiksy/internal/storage/disk"
	"eiksy/internal/storage/memory"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// App wires the Wails bridge to backend services.
type App struct {
	ctx     context.Context
	service *app.Service
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

	if store != nil {
		a.service = app.NewService(store, sshmanager.NewManager(), sftpmanager.NewManager())
	} else {
		a.service = app.NewService(memory.NewStore(), sshmanager.NewManager(), sftpmanager.NewManager())
	}
	a.service.SetRuntimeContext(ctx, func(eventName string, data ...interface{}) {
		runtime.EventsEmit(ctx, eventName, data...)
	})

	if storeErr != nil {
		a.service.EmitLog("warn", "Disk store unavailable; running with in-memory defaults.")
	} else {
		a.service.EmitLog("info", "Application started with disk storage.")
		a.autoImportInitialSSHConfig()
	}
}

// GetShellState returns the full backend-owned application state for the UI shell.
func (a *App) GetShellState() app.ShellState {
	return a.currentService().GetShellState()
}

func (a *App) GetReleaseVersion() string {
	return resolveReleaseVersion()
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

func (a *App) ListVaultSecretsForProvider(provider, path string) ([]vaultdomain.SecretNode, error) {
	return a.currentService().ListVaultSecretsForProvider(provider, path)
}

func (a *App) ImportSSHConfig(raw string) ([]sessions.Profile, error) {
	return a.currentService().ImportSSHConfig(raw)
}

func (a *App) OpenSessionWindow() error {
	return nil
}

func (a *App) GetSecureStorageStatus() securestorage.Status {
	return a.currentService().GetSecureStorageStatus()
}

func (a *App) EnsureMasterPassword(password string) error {
	return a.currentService().EnsureMasterPassword(password)
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

func (a *App) SaveLocalProvider(downloadURL string) error {
	return a.currentService().SaveLocalProvider(downloadURL)
}

func (a *App) DownloadLocalModel(downloadURL string) error {
	return a.currentService().DownloadLocalModel(downloadURL)
}

func (a *App) StartLocalModel() error {
	return a.currentService().StartLocalModel()
}

func (a *App) StopLocalModel() error {
	return a.currentService().StopLocalModel()
}

func (a *App) ListCloudModels(endpoint string, token string) ([]string, error) {
	return a.currentService().ListCloudModels(endpoint, token)
}

func (a *App) StartCloudProviderAuth(endpoint string) (app.CloudProviderAuthSession, error) {
	return a.currentService().StartCloudProviderAuth(endpoint)
}

func (a *App) GetCloudProviderAuthSession(sessionID string) (app.CloudProviderAuthSession, error) {
	return a.currentService().GetCloudProviderAuthSession(sessionID)
}

func (a *App) SendChatMessage(message string, activeSessionID string) error {
	return a.currentService().SendChatMessage(a.ctx, message, activeSessionID)
}

func (a *App) ClearChat() {
	a.currentService().ClearChat()
}

func (a *App) UpdateCommandPolicy(policy ai.CommandPolicy) error {
	return a.currentService().UpdateCommandPolicy(policy)
}

func (a *App) ResolveCommandPolicyRequest(requestID string, mode string) error {
	return a.currentService().ResolveCommandPolicyRequest(requestID, ai.CommandPermissionMode(mode))
}

func (a *App) AcceptSSHHostKey(tabID string) error {
	return a.currentService().AcceptSSHHostKey(tabID)
}

func (a *App) currentService() *app.Service {
	if a.service == nil {
		svc := app.NewService(memory.NewStore(), sshmanager.NewManager(), sftpmanager.NewManager())
		svc.SetRuntimeContext(a.ctx, func(eventName string, data ...interface{}) {
			runtime.EventsEmit(a.ctx, eventName, data...)
		})
		a.service = svc
	}
	return a.service
}

func (a *App) autoImportInitialSSHConfig() {
	state := a.service.GetShellState()
	if len(state.SessionProfiles) > 0 || state.Settings.SSHConfigAutoLoaded {
		return
	}
	settingsSnapshot := state.Settings
	settingsSnapshot.SSHConfigAutoLoaded = true
	if err := a.service.UpdateSettings(settingsSnapshot); err != nil {
		a.service.EmitLog("warn", "Unable to persist SSH auto-import marker.")
	}

	homeDir, err := os.UserHomeDir()
	if err != nil || homeDir == "" {
		return
	}
	configPath := filepath.Join(homeDir, ".ssh", "config")
	content, err := os.ReadFile(configPath)
	if err != nil {
		return
	}
	if len(content) == 0 {
		return
	}
	imported, err := a.service.ImportSSHConfig(string(content))
	if err != nil {
		a.service.EmitLog("warn", "Automatic SSH config import skipped: "+err.Error())
		return
	}
	a.service.EmitLog("info", "Automatically imported "+strconv.Itoa(len(imported))+" SSH session(s) from ~/.ssh/config.")
}
