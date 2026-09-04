import './style.css';
import './app.css';
import '@xterm/xterm/css/xterm.css';

import { Terminal } from '@xterm/xterm';
import { FitAddon } from '@xterm/addon-fit';
import { WebLinksAddon } from '@xterm/addon-web-links';
import {
    AcceptSSHHostKey,
    ClearChat,
    CloseSession,
    ConnectSSH,
    CreateSessionProfile,
    DownloadSFTPFiles,
    DeleteSessionProfile,
    DisconnectSSH,
    GetShellState,
    OpenRDP,
    LaunchSession,
    ListSFTPFiles,
    ListVaultSecrets,
    NavigateSFTP,
    ImportSSHConfig,
    ReadSFTPFile,
    ListCloudModels,
    SaveCloudProvider,
    SaveSFTPFile,
    SelectDownloadDirectory,
    SelectUploadFiles,
    SendChatMessage,
    SendSSHInput,
    ResizeTerminal,
    UploadSFTPFiles,
    UpdateSettings,
} from '../wailsjs/go/main/App';
import { EventsOn } from '../wailsjs/runtime/runtime';
import type { ai as aiModels, app as appModels, sessions, settings as settingsModels, sftp as sftpModels, vault as vaultModels } from '../wailsjs/go/models';

type ShellState = appModels.ShellState;
type RuntimeSession = appModels.RuntimeSessionView;
type SessionProfile = sessions.ProfileInput;
type AIProvider = aiModels.ProviderDescriptor;
type FileEntry = sftpModels.FileEntry;
type VaultSecretNode = vaultModels.SecretNode;

type SessionFormState = {
    name: string;
    host: string;
    port: string;
    username: string;
    password: string;
    authMethod: 'password' | 'key';
    privateKeyPath: string;
    protocolId: string;
    tags: string[];
    proxyJump: string;
    localForwards: string;
    useSSHAgent: boolean;
};

type SFTPState = {
    tabId: string | null;
    path: string;
    entries: FileEntry[];
    loading: boolean;
    error: string;
    editorOpen: boolean;
    editorPath: string;
    editorContent: string;
    editorLoading: boolean;
    editorSaving: boolean;
    editorDirty: boolean;
    editorError: string;
    selectedFiles: string[];
};

type VaultState = {
    path: string;
    entries: VaultSecretNode[];
    loading: boolean;
    error: string;
    loaded: boolean;
};

type TerminalState = {
    terminal: Terminal;
    fitAddon: FitAddon;
    wrapper: HTMLDivElement;
    inner: HTMLDivElement;
    opened: boolean;
    unsubscribe: (() => void) | null;
};

type Theme = 'dark' | 'light' | 'green';
type SettingsTab = 'ai' | 'vault' | 'sshconfig' | 'portforward' | 'theme';
type SessionModalTab = 'host' | 'auth' | 'network' | 'other';
type RightPanelTab = 'ai' | 'vault';
type SessionInnerTab = 'console' | 'sftp' | 'screen';

type PortForwardRule = {
    ports?: string;
    localPort: string;
    remoteHost: string;
    remotePort: string;
    hostId: string;
    enabled: boolean;
};

type NotificationLevel = 'info' | 'warn' | 'error' | 'debug';

type NotificationItem = {
    id: string;
    level: NotificationLevel;
    message: string;
    time: string;
};

type SessionContextMenuState = {
    visible: boolean;
    x: number;
    y: number;
    profileId: string;
};

type HostKeyDialogState = {
    visible: boolean;
    tabId: string;
    profileId: string;
    fingerprint: string;
    hostname: string;
};

const THEME_KEY = 'opsy-theme';
const THEMES: Theme[] = ['dark', 'light', 'green'];

function isTheme(value: string | null | undefined): value is Theme {
    return !!value && THEMES.includes(value as Theme);
}

const root = document.querySelector<HTMLDivElement>('#app');

class OpsyShell {
    private readonly untaggedFilterTag = '__untagged__';
    private shellState: ShellState | null = null;
    private activeTabId = '';
    private errorMessage = '';
    private showSessionModal = false;
    private sessionModalTab: SessionModalTab = 'host';
    private editingProfileID = '';
    private sessionNameAuto = true;
    private showSettingsModal = false;
    private settingsTab: SettingsTab = 'ai';
    private rightPanelTab: RightPanelTab = 'ai';
    private sessionInnerTab: SessionInnerTab = 'console';
    private sessionForm: SessionFormState = this.defaultSessionForm();
    private terminals = new Map<string, TerminalState>();
    private sftpState: SFTPState = {
        tabId: null,
        path: '',
        entries: [],
        loading: false,
        error: '',
        editorOpen: false,
        editorPath: '',
        editorContent: '',
        editorLoading: false,
        editorSaving: false,
        editorDirty: false,
        editorError: '',
        selectedFiles: [],
    };
    private sshConfigDraft = '';
    private vaultState: VaultState = { path: '', entries: [], loading: false, error: '', loaded: false };
    private theme: Theme;
    private sessionContextMenu: SessionContextMenuState = { visible: false, x: 0, y: 0, profileId: '' };
    private hostKeyDialog: HostKeyDialogState = { visible: false, tabId: '', profileId: '', fingerprint: '', hostname: '' };
    private selectedSessionTags = new Set<string>();
    private knownSessionTags = new Set<string>();
    private sessionTagFilterInitialized = false;
    private sessionTagDraft = '';
    private sessionTagInputVisible = false;
    private includeLastCommandOutput = false;
    private terminalOutputHistory = new Map<string, string>();
    private aiStatus: 'idle' | 'thinking' = 'idle';
    private cloudModels: string[] = [];
    private cloudModelsEndpoint = '';
    private cloudModelsLoading = false;
    private cloudModelsError = '';
    private pfNewLocalPort = '';
    private pfNewRemoteHost = '';
    private pfNewRemotePort = '';
    private pfNewHostId = '';
    private cloudDraftModel = '';
    private cloudDraftEndpoint = '';
    private cloudDraftToken = '';
    private vaultDraftAddress = '';
    private vaultDraftMountPoint = '';
    private vaultDraftToken = '';
    private vaultDraftAutoRenewToken = false;
    private vaultDraftProvider = 'vault';
    private vaultDraftKeePassDatabasePath = '';
    private vaultDraftKeePassPassword = '';
    private notifications: NotificationItem[] = [];
    private toastQueue: NotificationItem[] = [];
    private showNotificationCenter = false;

    constructor() {
        const saved = localStorage.getItem(THEME_KEY);
        this.theme = isTheme(saved) ? saved : 'dark';
        this.applyTheme();
    }

    private applyTheme(): void {
        document.documentElement.classList.remove('light', 'green');
        if (this.theme !== 'dark') {
            document.documentElement.classList.add(this.theme);
        }
        localStorage.setItem(THEME_KEY, this.theme);
    }

    async bootstrap(): Promise<void> {
        if (!root) {
            return;
        }

        this.registerGlobalEvents();
        await this.refresh();
        window.addEventListener('resize', () => this.fitActiveTerminal());
    }

    private registerGlobalEvents(): void {
        EventsOn('app:log', (...payload: unknown[]) => {
            const data = payload[0] as { level?: string; message?: string; time?: string } | undefined;
            if (!data?.message) return;
            const level = this.normalizeNotificationLevel(data.level);
            this.pushNotification(level, data.message, data.time ?? new Date().toISOString());
        });
        EventsOn('ai:status', (...payload: unknown[]) => {
            const data = payload[0] as { status?: string } | undefined;
            this.aiStatus = data?.status === 'thinking' ? 'thinking' : 'idle';
            this.render();
        });
        EventsOn('model:error', (...payload: unknown[]) => {
            const data = payload[0] as { error?: string } | undefined;
            if (!data?.error) {
                return;
            }
            this.setErrorMessage(`Model error: ${data.error}`);
        });
        EventsOn('ai:message', () => {
            void this.refresh('');
        });
    }

    private async refresh(errorMessage = this.errorMessage): Promise<void> {
        this.errorMessage = errorMessage;
        this.shellState = await GetShellState();
        this.reconcileSelectedSessionTags();
        const preferredTabID = this.activeTabId || this.shellState.workspace.layout.activeTabId || '';
        this.activeTabId = this.pickActiveTabID(preferredTabID);
        const activeTab = this.activeTab();
        this.sessionInnerTab = activeTab
            ? this.normalizeSessionInnerTab(activeTab.protocolId, this.sessionInnerTab)
            : 'console';
        if (this.sftpState.tabId !== this.activeTabId) {
            this.sftpState = this.defaultSFTPState(this.activeTabId || null);
        }
        this.render();
    }

    private closeRemoteEditor(): void {
        this.sftpState = {
            ...this.sftpState,
            editorOpen: false,
            editorPath: '',
            editorContent: '',
            editorLoading: false,
            editorSaving: false,
            editorDirty: false,
            editorError: '',
        };
        this.render();
    }

    private render(): void {
        if (!root || !this.shellState) {
            return;
        }

        root.innerHTML = `
            <div class="shell-root">
                <div class="shell">
                    <aside class="panel sidebar-panel">
                        <div class="panel-header">
                            <div>
                                <div class="eyebrow">Workspace</div>
                                <h1>opsy</h1>
                            </div>
                            <div style="display:flex;gap:0.5rem;align-items:center;">
                                <button class="icon-button" data-open-session-modal title="New session">+</button>
                                <button class="icon-button" data-open-notification-center title="Notifications">🔔${this.notifications.length > 0 ? ` ${this.notifications.length}` : ''}</button>
                                <button class="icon-button" data-open-settings-modal title="Settings">⚙</button>
                            </div>
                        </div>
                        <div class="sidebar-body">
                            <section class="section sidebar-section sessions-section">
                                <div class="section-heading">
                                    <span class="section-title">Sessions</span>
                                </div>
                                <div class="section-copy">Double-click a session to open it. Right-click to manage.</div>
                                <div class="session-list">
                                    ${this.renderSessionProfiles()}
                                </div>
                            </section>
                            <section class="section sidebar-section session-tags-filter-section">
                                <div class="section-heading">
                                    <span class="section-title">Tags</span>
                                </div>
                                ${this.renderSessionTagFilters()}
                            </section>
                        </div>
                    </aside>

                    <main class="panel workspace-panel">
                        <div class="tab-bar">${this.renderTabs()}</div>
                        ${this.errorMessage ? `<div class="error-banner">${escapeHtml(this.errorMessage)}</div>` : ''}
                        ${this.activeTab() ? `
                        <div class="session-inner-tabs">
                            ${this.renderSessionInnerTabs()}
                        </div>
                        ` : ''}
                        <div class="terminal-shell ${this.sessionInnerTab === 'console' ? '' : 'hidden'}">
                            <div id="terminal-host" class="terminal-container"></div>
                        </div>
                        <div class="sftp-workspace ${this.sessionInnerTab === 'sftp' ? '' : 'hidden'}">
                            <div class="sftp-workspace-header">
                                <div class="section-actions">
                                    ${this.renderSFTPActions()}
                                </div>
                            </div>
                            ${this.renderSFTPBrowser()}
                        </div>
                        <div class="screen-workspace ${this.sessionInnerTab === 'screen' ? '' : 'hidden'}">
                            ${this.renderScreenWorkspace()}
                        </div>
                    </main>

                    <aside class="panel assistant-panel">
                        <div class="panel-header">
                            <div>
                                <div class="eyebrow">Assistant</div>
                                <h2>${this.rightPanelTab === 'ai' ? 'AI' : 'Vault browser'}</h2>
                            </div>
                            ${this.rightPanelTab === 'ai' && this.hasConfiguredProvider() ? `<button class="icon-button" data-clear-chat title="Clear chat">🗑</button>` : ''}
                        </div>
                        <div class="sidebar-panel-tabs">
                            <button class="panel-tab ${this.rightPanelTab === 'ai' ? 'active' : ''}" data-right-panel-tab="ai">AI</button>
                            <button class="panel-tab ${this.rightPanelTab === 'vault' ? 'active' : ''}" data-right-panel-tab="vault">Vault browser</button>
                        </div>
                        ${this.rightPanelTab === 'ai' ? `
                        <section class="section chat-section">
                            <div class="section-title">Assistant</div>
                            <div class="chat-messages" id="chat-messages">
                                ${this.renderMessages()}
                            </div>
                        </section>
                        ${this.hasConfiguredProvider() ? `
                        <form class="chat-input-form" data-chat-form>
                            ${this.aiStatus === 'thinking' ? '<div class="ai-status-indicator">⏳ Thinking…</div>' : ''}
                            <textarea class="chat-textarea" name="message" placeholder="Ask the assistant… (Ctrl+Enter to send)" rows="3"></textarea>
                            <label class="inline-check chat-attach-row">
                                <span>Attach latest console output</span>
                                <input name="includeLastOutput" type="checkbox" ${this.includeLastCommandOutput ? 'checked' : ''} />
                            </label>
                            <button class="action-button" type="submit" ${this.aiStatus === 'thinking' ? 'disabled' : ''}>Send</button>
                        </form>
                        ` : ''}` : `
                        <section class="section chat-section">
                            <div class="section-heading">
                                <span class="section-title">Vault Tree</span>
                                <div class="section-actions">
                                    <button class="action-button secondary" data-open-vault>Open</button>
                                    <button class="action-button secondary" data-refresh-vault>Refresh</button>
                                </div>
                            </div>
                            ${this.renderVaultBrowser()}
                        </section>`}
                    </aside>
                </div>
            </div>
            ${this.renderSessionContextMenu()}
            ${this.hostKeyDialog.visible ? this.renderHostKeyDialog() : ''}
            ${this.renderRemoteEditorModal()}
            ${this.showSessionModal ? this.renderSessionModal() : ''}
            ${this.showSettingsModal ? this.renderSettingsModal() : ''}
            ${this.showNotificationCenter ? this.renderNotificationCenter() : ''}
            ${this.renderToasts()}
        `;

        this.bindEvents();
        this.attachActiveTerminal();
        this.scrollChatToBottom();
    }

    private bindEvents(): void {
        root?.querySelector<HTMLDivElement>('[data-session-context-overlay]')?.addEventListener('click', () => {
            this.hideSessionContextMenu();
        });
        root?.querySelectorAll<HTMLButtonElement>('[data-session-context-open]').forEach((button) => {
            button.addEventListener('click', async () => {
                const profileID = button.dataset.sessionContextOpen;
                this.hideSessionContextMenu();
                if (profileID) {
                    await this.openProfile(profileID);
                }
            });
        });
        root?.querySelectorAll<HTMLButtonElement>('[data-session-context-delete]').forEach((button) => {
            button.addEventListener('click', async () => {
                const profileID = button.dataset.sessionContextDelete;
                this.hideSessionContextMenu();
                if (profileID) {
                    await this.runAction(async () => DeleteSessionProfile(profileID), 'Unable to delete session profile');
                }
            });
        });
        root?.querySelectorAll<HTMLButtonElement>('[data-session-context-edit]').forEach((button) => {
            button.addEventListener('click', () => {
                const profileID = button.dataset.sessionContextEdit;
                this.hideSessionContextMenu();
                if (profileID) {
                    this.openSessionModalForEdit(profileID);
                }
            });
        });

        root?.querySelector<HTMLButtonElement>('[data-open-session-modal]')?.addEventListener('click', () => {
            this.openSessionModalForCreate();
        });

        root?.querySelector<HTMLButtonElement>('[data-open-settings-modal]')?.addEventListener('click', () => {
            this.showSettingsModal = true;
            this.initializeSettingsDrafts();
            this.render();
            if (this.settingsTab === 'ai') {
                void this.loadCloudModels(false);
            }
        });
        root?.querySelector<HTMLButtonElement>('[data-open-notification-center]')?.addEventListener('click', () => {
            this.showNotificationCenter = true;
            this.render();
        });

        root?.querySelectorAll<HTMLButtonElement>('[data-session-inner-tab]').forEach((button) => {
            button.addEventListener('click', async () => {
                this.sessionInnerTab = (button.dataset.sessionInnerTab as SessionInnerTab) ?? 'console';
                this.render();
                if (this.sessionInnerTab === 'sftp') {
                    await this.ensureActiveSFTPLoaded();
                }
            });
        });

        root?.querySelectorAll<HTMLButtonElement>('[data-right-panel-tab]').forEach((button) => {
            button.addEventListener('click', () => {
                this.rightPanelTab = (button.dataset.rightPanelTab as RightPanelTab) ?? 'ai';
                this.render();
            });
        });

        root?.querySelectorAll<HTMLButtonElement>('[data-settings-tab]').forEach((button) => {
            button.addEventListener('click', () => {
                this.settingsTab = (button.dataset.settingsTab as SettingsTab) ?? 'ai';
                this.render();
                if (this.settingsTab === 'ai') {
                    void this.loadCloudModels(false);
                }
            });
        });

        root?.querySelectorAll<HTMLButtonElement>('[data-session-modal-tab]').forEach((button) => {
            button.addEventListener('click', () => {
                this.syncSessionFormFromDOM();
                this.sessionModalTab = (button.dataset.sessionModalTab as SessionModalTab) ?? 'host';
                this.render();
            });
        });
        root?.querySelector<HTMLFormElement>('[data-session-form]')?.addEventListener('input', (event) => {
            this.syncSessionFormFromDOM(event.target);
        });
        root?.querySelector<HTMLSelectElement>('select[name="protocolId"]')?.addEventListener('change', (event) => {
            const select = event.currentTarget as HTMLSelectElement;
            const previousProtocolID = this.sessionForm.protocolId;
            this.syncSessionFormFromDOM();
            this.sessionForm.protocolId = select.value || 'ssh';
            if (this.sessionForm.protocolId === 'rdp') {
                this.sessionForm.authMethod = 'password';
                if (!this.sessionForm.port || this.sessionForm.port === '22' || previousProtocolID !== 'rdp') {
                    this.sessionForm.port = '3389';
                }
            } else if (previousProtocolID === 'rdp' && (!this.sessionForm.port || this.sessionForm.port === '3389')) {
                this.sessionForm.port = '22';
            }
            this.render();
        });
        root?.querySelector<HTMLSelectElement>('select[name="authMethod"]')?.addEventListener('change', (event) => {
            this.syncSessionFormFromDOM();
            this.sessionForm.authMethod = ((event.currentTarget as HTMLSelectElement).value === 'key' ? 'key' : 'password');
            this.render();
        });

        root?.querySelectorAll<HTMLElement>('[data-session-item]').forEach((item) => {
            item.addEventListener('dblclick', async () => {
                const profileID = item.dataset.sessionItem;
                if (!profileID) {
                    return;
                }
                await this.openProfile(profileID);
            });
            item.addEventListener('contextmenu', (event) => {
                event.preventDefault();
                const profileID = item.dataset.sessionItem;
                if (!profileID) {
                    return;
                }
                this.sessionContextMenu = {
                    visible: true,
                    x: event.clientX,
                    y: event.clientY,
                    profileId: profileID,
                };
                this.render();
            });
        });

        root?.querySelector<HTMLTextAreaElement>('[data-ssh-config-import]')?.addEventListener('input', (event) => {
            this.sshConfigDraft = (event.currentTarget as HTMLTextAreaElement).value;
        });

        root?.querySelector<HTMLButtonElement>('[data-import-ssh-config]')?.addEventListener('click', async () => {
            try {
                await ImportSSHConfig(this.sshConfigDraft);
                this.sshConfigDraft = '';
                await this.refresh('');
            } catch (error) {
                this.setErrorMessage(formatError('Unable to import SSH config', error));
                this.render();
            }
        });

        root?.querySelectorAll<HTMLButtonElement>('[data-tab-id]').forEach((button) => {
            button.addEventListener('click', async () => {
                const tabID = button.dataset.tabId;
                if (!tabID) {
                    return;
                }
                this.activeTabId = tabID;
                const activeTab = this.activeTab();
                this.sessionInnerTab = activeTab
                    ? this.normalizeSessionInnerTab(activeTab.protocolId, this.sessionInnerTab)
                    : 'console';
                if (this.sftpState.tabId !== tabID) {
                    this.sftpState = this.defaultSFTPState(tabID);
                }
                this.render();
                if (this.sessionInnerTab === 'sftp') {
                    await this.ensureActiveSFTPLoaded();
                }
            });
        });

        root?.querySelectorAll<HTMLButtonElement>('[data-close-tab]').forEach((button) => {
            button.addEventListener('click', async (event) => {
                event.stopPropagation();
                const tabID = button.dataset.closeTab;
                if (!tabID) {
                    return;
                }
                await this.closeTab(tabID);
            });
        });

        root?.querySelector<HTMLButtonElement>('[data-refresh-sftp]')?.addEventListener('click', async () => {
            const activeTab = this.activeTab();
            if (!activeTab) {
                return;
            }
            await this.loadSFTP(activeTab.id, this.sftpState.path);
        });

        root?.querySelector<HTMLButtonElement>('[data-sftp-up]')?.addEventListener('click', async () => {
            const activeTab = this.activeTab();
            if (!activeTab) {
                return;
            }
            await this.loadSFTP(activeTab.id, parentPath(this.sftpState.path));
        });
        root?.querySelector<HTMLButtonElement>('[data-sftp-upload]')?.addEventListener('click', async () => {
            const activeTab = this.activeTab();
            if (!activeTab) {
                return;
            }
            await this.uploadToSFTP(activeTab.id);
        });
        root?.querySelector<HTMLButtonElement>('[data-sftp-download]')?.addEventListener('click', async () => {
            const activeTab = this.activeTab();
            if (!activeTab) {
                return;
            }
            await this.downloadFromSFTP(activeTab.id);
        });

        root?.querySelector<HTMLButtonElement>('[data-open-vault]')?.addEventListener('click', async () => {
            await this.loadVaultSecrets('');
        });

        root?.querySelector<HTMLButtonElement>('[data-refresh-vault]')?.addEventListener('click', async () => {
            await this.loadVaultSecrets(this.vaultState.path);
        });

        root?.querySelector<HTMLButtonElement>('[data-vault-up]')?.addEventListener('click', async () => {
            await this.loadVaultSecrets(parentVaultPath(this.vaultState.path));
        });

        root?.querySelectorAll<HTMLButtonElement>('[data-vault-dir]').forEach((button) => {
            button.addEventListener('click', async () => {
                const targetPath = button.dataset.vaultDir;
                if (!targetPath) return;
                await this.loadVaultSecrets(targetPath);
            });
        });

        root?.querySelectorAll<HTMLButtonElement>('[data-sftp-dir]').forEach((button) => {
            button.addEventListener('click', async () => {
                const targetPath = button.dataset.sftpDir;
                const activeTab = this.activeTab();
                if (!targetPath || !activeTab) {
                    return;
                }
                await this.loadSFTP(activeTab.id, targetPath);
            });
        });
        root?.querySelectorAll<HTMLButtonElement>('[data-sftp-file-edit]').forEach((button) => {
            button.addEventListener('click', async () => {
                const targetPath = button.dataset.sftpFileEdit;
                const activeTab = this.activeTab();
                if (!targetPath || !activeTab) {
                    return;
                }
                await this.openRemoteFile(activeTab.id, targetPath);
            });
        });
        root?.querySelectorAll<HTMLInputElement>('[data-sftp-select-file]').forEach((input) => {
            input.addEventListener('change', () => {
                const targetPath = input.dataset.sftpSelectFile;
                if (!targetPath) {
                    return;
                }
                this.toggleSFTPFileSelection(targetPath, input.checked);
            });
        });
        root?.querySelector<HTMLButtonElement>('[data-save-remote-file]')?.addEventListener('click', async () => {
            const activeTab = this.activeTab();
            if (!activeTab || !this.sftpState.editorPath) {
                return;
            }
            await this.saveRemoteFile(activeTab.id);
        });
        root?.querySelector<HTMLButtonElement>('[data-close-remote-editor]')?.addEventListener('click', () => {
            this.closeRemoteEditor();
        });
        root?.querySelector<HTMLTextAreaElement>('[data-remote-editor]')?.addEventListener('input', (event) => {
            this.sftpState.editorContent = (event.currentTarget as HTMLTextAreaElement).value;
            this.sftpState.editorDirty = true;
        });

        root?.querySelector<HTMLFormElement>('[data-cloud-form]')?.addEventListener('submit', async (event) => {
            event.preventDefault();
            const model = this.cloudDraftModel;
            const endpoint = this.cloudDraftEndpoint;
            const token = this.cloudDraftToken;
            await this.runAction(async () => SaveCloudProvider(model, endpoint, token), 'Unable to save cloud provider');
        });
        root?.querySelector<HTMLInputElement>('[data-cloud-model]')?.addEventListener('input', (event) => {
            this.cloudDraftModel = (event.currentTarget as HTMLInputElement).value;
        });
        root?.querySelector<HTMLInputElement>('[data-cloud-endpoint]')?.addEventListener('input', (event) => {
            this.cloudDraftEndpoint = (event.currentTarget as HTMLInputElement).value;
        });
        root?.querySelector<HTMLInputElement>('[data-cloud-token]')?.addEventListener('input', (event) => {
            this.cloudDraftToken = (event.currentTarget as HTMLInputElement).value;
        });
        root?.querySelector<HTMLButtonElement>('[data-load-cloud-models]')?.addEventListener('click', async () => {
            await this.loadCloudModels(true);
        });

        root?.querySelector<HTMLFormElement>('[data-chat-form]')?.addEventListener('submit', async (event) => {
            event.preventDefault();
            const form = event.currentTarget as HTMLFormElement;
            const textarea = form.querySelector<HTMLTextAreaElement>('[name="message"]');
            const message = textarea?.value ?? '';
            if (!message.trim()) return;
            if (textarea) textarea.value = '';
            const includeLastOutput = form.querySelector<HTMLInputElement>('input[name="includeLastOutput"]')?.checked ?? false;
            this.includeLastCommandOutput = includeLastOutput;
            const payload = includeLastOutput ? this.withLatestTerminalOutput(message) : message;
            await this.runAction(async () => SendChatMessage(payload), 'Unable to send message');
        });

        root?.querySelector<HTMLTextAreaElement>('[data-chat-form] textarea[name="message"]')?.addEventListener('keydown', (event) => {
            if (event.ctrlKey && event.key === 'Enter') {
                event.preventDefault();
                const form = root?.querySelector<HTMLFormElement>('[data-chat-form]');
                form?.requestSubmit();
            }
        });

        root?.querySelector<HTMLButtonElement>('[data-clear-chat]')?.addEventListener('click', async () => {
            try {
                await ClearChat();
                this.aiStatus = 'idle';
                await this.refresh('');
            } catch (error) {
                this.setErrorMessage(formatError('Unable to clear chat', error));
                this.render();
            }
        });

        root?.querySelectorAll<HTMLElement>('[data-close-modal]').forEach((button) => {
            button.addEventListener('click', () => {
                const wasSessionModalOpen = this.showSessionModal;
                this.showSessionModal = false;
                this.showSettingsModal = false;
                this.showNotificationCenter = false;
                if (wasSessionModalOpen) {
                    this.editingProfileID = '';
                    this.sessionForm = this.defaultSessionForm();
                    this.sessionTagDraft = '';
                    this.sessionTagInputVisible = false;
                    this.sessionNameAuto = true;
                    this.sessionModalTab = 'host';
                }
                this.render();
            });
        });

        root?.querySelectorAll<HTMLButtonElement>('[data-set-theme]').forEach((button) => {
            button.addEventListener('click', () => {
                const t = button.dataset.setTheme as Theme;
                if (isTheme(t)) {
                    this.theme = t;
                    this.applyTheme();
                    this.render();
                }
            });
        });

        root?.querySelector<HTMLFormElement>('[data-vault-settings-form]')?.addEventListener('submit', async (event) => {
            event.preventDefault();
            if (!this.shellState) return;
            const updated = {
                ...this.shellState.settings,
                vaultAddress: this.vaultDraftAddress,
                vaultMountPoint: this.vaultDraftMountPoint,
                vaultToken: this.vaultDraftToken,
                vaultAutoRenewToken: this.vaultDraftAutoRenewToken,
                vaultProvider: this.vaultDraftProvider,
                keepassDatabasePath: this.vaultDraftKeePassDatabasePath,
                keepassPassword: this.vaultDraftKeePassPassword,
            } as unknown as settingsModels.AppSettings;
            await this.runAction(async () => UpdateSettings(updated), 'Unable to save Vault settings');
        });
        root?.querySelector<HTMLSelectElement>('[data-vault-provider]')?.addEventListener('change', (event) => {
            this.vaultDraftProvider = (event.currentTarget as HTMLSelectElement).value || 'vault';
            this.render();
        });
        root?.querySelector<HTMLInputElement>('[data-vault-address]')?.addEventListener('input', (event) => {
            this.vaultDraftAddress = (event.currentTarget as HTMLInputElement).value;
        });
        root?.querySelector<HTMLInputElement>('[data-vault-mount]')?.addEventListener('input', (event) => {
            this.vaultDraftMountPoint = (event.currentTarget as HTMLInputElement).value;
        });
        root?.querySelector<HTMLInputElement>('[data-vault-token]')?.addEventListener('input', (event) => {
            this.vaultDraftToken = (event.currentTarget as HTMLInputElement).value;
        });
        root?.querySelector<HTMLInputElement>('[data-vault-renew]')?.addEventListener('change', (event) => {
            this.vaultDraftAutoRenewToken = (event.currentTarget as HTMLInputElement).checked;
        });
        root?.querySelector<HTMLInputElement>('[data-keepass-db-path]')?.addEventListener('input', (event) => {
            this.vaultDraftKeePassDatabasePath = (event.currentTarget as HTMLInputElement).value;
        });
        root?.querySelector<HTMLInputElement>('[data-keepass-password]')?.addEventListener('input', (event) => {
            this.vaultDraftKeePassPassword = (event.currentTarget as HTMLInputElement).value;
        });
        root?.querySelectorAll<HTMLButtonElement>('[data-toggle-session-tag-filter]').forEach((button) => {
            button.addEventListener('click', () => {
                const tag = String(button.dataset.toggleSessionTagFilter ?? '').trim();
                if (!tag) {
                    return;
                }
                this.toggleSessionTagFilter(tag);
            });
        });

        root?.querySelector<HTMLFormElement>('[data-pf-add-form]')?.addEventListener('submit', async (event) => {
            event.preventDefault();
            if (!this.shellState) return;
            const form = event.currentTarget as HTMLFormElement;
            const localPort = (form.querySelector<HTMLInputElement>('input[name="pfLocalPort"]')?.value ?? '').trim();
            const remoteHost = (form.querySelector<HTMLInputElement>('input[name="pfRemoteHost"]')?.value ?? '').trim();
            const remotePort = (form.querySelector<HTMLInputElement>('input[name="pfRemotePort"]')?.value ?? '').trim();
            const hostId = (form.querySelector<HTMLSelectElement>('select[name="pfHostId"]')?.value ?? '').trim();
            if (!localPort || !hostId || !remoteHost || !remotePort) return;
            const rules: PortForwardRule[] = [...(this.shellState.settings.portForwardRules ?? []), { localPort, remoteHost, remotePort, hostId, enabled: true }];
            const updated = { ...this.shellState.settings, portForwardRules: rules } as unknown as settingsModels.AppSettings;
            this.pfNewLocalPort = '';
            this.pfNewRemoteHost = '';
            this.pfNewRemotePort = '';
            this.pfNewHostId = '';
            await this.runAction(async () => UpdateSettings(updated), 'Unable to save port forwarding rule');
        });
        root?.querySelectorAll<HTMLButtonElement>('[data-pf-start-stop]').forEach((button) => {
            button.addEventListener('click', async () => {
                if (!this.shellState) return;
                const idx = Number(button.dataset.pfStartStop);
                const rules: PortForwardRule[] = (this.shellState.settings.portForwardRules ?? []).map((r, i) =>
                    i === idx ? { ...r, enabled: !r.enabled } : r
                );
                const updated = { ...this.shellState.settings, portForwardRules: rules } as unknown as settingsModels.AppSettings;
                await this.runAction(async () => UpdateSettings(updated), 'Unable to update port forwarding rule state');
            });
        });
        root?.querySelectorAll<HTMLButtonElement>('[data-pf-delete]').forEach((button) => {
            button.addEventListener('click', async () => {
                if (!this.shellState) return;
                const idx = Number(button.dataset.pfDelete);
                const rules: PortForwardRule[] = (this.shellState.settings.portForwardRules ?? []).filter((_, i) => i !== idx);
                const updated = { ...this.shellState.settings, portForwardRules: rules } as unknown as settingsModels.AppSettings;
                await this.runAction(async () => UpdateSettings(updated), 'Unable to delete port forwarding rule');
            });
        });
        root?.querySelector<HTMLInputElement>('input[name="pfLocalPort"]')?.addEventListener('input', (event) => {
            this.pfNewLocalPort = (event.currentTarget as HTMLInputElement).value;
        });
        root?.querySelector<HTMLInputElement>('input[name="pfRemoteHost"]')?.addEventListener('input', (event) => {
            this.pfNewRemoteHost = (event.currentTarget as HTMLInputElement).value;
        });
        root?.querySelector<HTMLInputElement>('input[name="pfRemotePort"]')?.addEventListener('input', (event) => {
            this.pfNewRemotePort = (event.currentTarget as HTMLInputElement).value;
        });
        root?.querySelector<HTMLSelectElement>('select[name="pfHostId"]')?.addEventListener('change', (event) => {
            this.pfNewHostId = (event.currentTarget as HTMLSelectElement).value;
        });

        root?.querySelector<HTMLFormElement>('[data-session-form]')?.addEventListener('submit', async (event) => {
            event.preventDefault();
            const form = event.currentTarget as HTMLFormElement;
            const formData = new FormData(form);
            this.commitSessionTagDraft();
            const protocolId = String(formData.get('protocolId') ?? 'ssh');
            const authMethod = protocolId === 'rdp' ? 'password' : (String(formData.get('authMethod') ?? 'password') === 'key' ? 'key' : 'password');
            const privateKeyPath = String(formData.get('privateKeyPath') ?? '');
            const host = String(formData.get('host') ?? '').trim();
            const name = String(formData.get('name') ?? '').trim() || host;
            const profile: SessionProfile = {
                id: this.editingProfileID,
                name,
                group: '',
                host,
                port: Number(formData.get('port') ?? (protocolId === 'rdp' ? 3389 : 22)),
                username: String(formData.get('username') ?? ''),
                password: String(authMethod === 'password' ? (formData.get('password') ?? '') : ''),
                protocolId,
                tags: this.sessionForm.tags,
                favorite: false,
            };
            const proxyJump = String(formData.get('proxyJump') ?? '').trim();
            const localForwards = String(formData.get('localForwards') ?? '').trim();
            const useSSHAgent = formData.get('useSSHAgent') === 'on';
            const keyPath = privateKeyPath.trim();
            if (authMethod === 'key' && !keyPath) {
                this.setErrorMessage('Unable to save session profile: private key path is required for key auth');
                this.render();
                return;
            }
            const options: Record<string, string> = {};
            options.auth_method = authMethod;
            if (authMethod === 'key' && keyPath) {
                options.ssh_private_key_path = keyPath;
            }
            if (this.supportsSSHAdvancedOptions(protocolId) && proxyJump) {
                options.proxy_jump = proxyJump;
            }
            if (this.supportsSSHAdvancedOptions(protocolId) && localForwards) {
                options.local_forwards = localForwards;
            }
            if (this.supportsSSHAdvancedOptions(protocolId) && useSSHAgent) {
                options.use_ssh_agent = 'true';
            }
            if (Object.keys(options).length > 0) {
                profile.options = options;
            }
            try {
                await CreateSessionProfile(profile);
                this.showSessionModal = false;
                this.editingProfileID = '';
                this.sessionForm = this.defaultSessionForm();
                this.sessionTagDraft = '';
                this.sessionTagInputVisible = false;
                this.sessionNameAuto = true;
                this.sessionModalTab = 'host';
                await this.refresh('');
            } catch (error) {
                this.setErrorMessage(formatError('Unable to save session profile', error));
                this.render();
            }
        });

        root?.querySelector<HTMLButtonElement>('[data-accept-host-key]')?.addEventListener('click', async () => {
            const { tabId, profileId } = this.hostKeyDialog;
            this.hostKeyDialog = { visible: false, tabId: '', profileId: '', fingerprint: '', hostname: '' };
            try {
                await AcceptSSHHostKey(tabId);
                await this.retrySSHConnect(tabId, profileId);
            } catch (error) {
                this.setErrorMessage(formatError('Unable to accept host key', error));
                this.render();
            }
        });

        root?.querySelector<HTMLButtonElement>('[data-reject-host-key]')?.addEventListener('click', () => {
            this.hostKeyDialog = { visible: false, tabId: '', profileId: '', fingerprint: '', hostname: '' };
            this.render();
        });

        root?.querySelector<HTMLButtonElement>('[data-session-tag-add-open]')?.addEventListener('click', () => {
            this.sessionTagInputVisible = true;
            this.render();
            requestAnimationFrame(() => {
                root?.querySelector<HTMLInputElement>('[data-session-tag-input]')?.focus();
            });
        });

        root?.querySelector<HTMLInputElement>('[data-session-tag-input]')?.addEventListener('input', (event) => {
            this.sessionTagDraft = (event.currentTarget as HTMLInputElement).value;
        });

        root?.querySelector<HTMLInputElement>('[data-session-tag-input]')?.addEventListener('keydown', (event) => {
            if (event.key !== 'Enter') {
                return;
            }
            event.preventDefault();
            this.commitSessionTagDraft();
            this.render();
        });

        root?.querySelector<HTMLInputElement>('[data-session-tag-input]')?.addEventListener('blur', () => {
            this.commitSessionTagDraft();
            requestAnimationFrame(() => this.render());
        });
        root?.querySelector<HTMLButtonElement>('[data-clear-notifications]')?.addEventListener('click', () => {
            this.notifications = [];
            this.toastQueue = [];
            this.render();
        });
        root?.querySelectorAll<HTMLButtonElement>('[data-delete-notification]').forEach((button) => {
            button.addEventListener('click', () => {
                const id = String(button.dataset.deleteNotification ?? '');
                if (!id) {
                    return;
                }
                this.notifications = this.notifications.filter((notification) => notification.id !== id);
                this.toastQueue = this.toastQueue.filter((notification) => notification.id !== id);
                this.render();
            });
        });

        root?.querySelectorAll<HTMLButtonElement>('[data-session-tag-remove]').forEach((button) => {
            button.addEventListener('click', () => {
                const tag = String(button.dataset.sessionTagRemove ?? '').trim();
                if (!tag) {
                    return;
                }
                this.sessionForm.tags = this.sessionForm.tags.filter((entry) => entry !== tag);
                this.render();
            });
        });
    }

    private async openProfile(profileID: string): Promise<void> {
        const profile = this.shellState?.sessionProfiles.find((entry) => entry.id === profileID);
        if (!profile) {
            return;
        }
        try {
            const tab = await LaunchSession(profileID);
            this.activeTabId = tab.id;
            this.sessionInnerTab = this.defaultSessionInnerTab(profile.protocolId);
            await this.refresh('');
            if (profile.protocolId === 'ssh') {
                this.ensureTerminalSubscription(tab.id);
                if (await this.connectSSHWithHostKeyHandling(tab.id, profileID)) {
                    this.fitActiveTerminal();
                    await this.ensureActiveSFTPLoaded(true);
                }
            } else if (profile.protocolId === 'rdp') {
                await OpenRDP(tab.id, profileID);
                this.errorMessage = '';
                this.render();
            }
        } catch (error) {
            this.setErrorMessage(formatError('Unable to open session', error));
            this.render();
        }
    }

    private async connectSSHWithHostKeyHandling(tabId: string, profileId: string): Promise<boolean> {
        try {
            await ConnectSSH(tabId, profileId);
            return true;
        } catch (error) {
            const msg = error instanceof Error ? error.message : String(error);
            if (msg.includes('unknown host key')) {
                const fpMatch = msg.match(/fingerprint\s+(\S+)/);
                const hostMatch = msg.match(/unknown host key:\s*(\S+)/);
                this.hostKeyDialog = {
                    visible: true,
                    tabId,
                    profileId,
                    fingerprint: fpMatch ? fpMatch[1] : '',
                    hostname: hostMatch ? hostMatch[1] : '',
                };
                this.render();
                return false;
            }
            throw error;
        }
    }

    private async retrySSHConnect(tabId: string, profileId: string): Promise<void> {
        try {
            this.ensureTerminalSubscription(tabId);
            await ConnectSSH(tabId, profileId);
            this.fitActiveTerminal();
            await this.refresh('');
            await this.ensureActiveSFTPLoaded(true);
        } catch (error) {
            this.setErrorMessage(formatError('Unable to connect SSH', error));
            this.render();
        }
    }

    private async closeTab(tabID: string): Promise<void> {
        const terminal = this.terminals.get(tabID);
        terminal?.unsubscribe?.();
        terminal?.terminal.dispose();
        terminal?.wrapper.remove();
        this.terminals.delete(tabID);
        this.terminalOutputHistory.delete(tabID);
        try {
            await DisconnectSSH(tabID);
        } catch {
            // ignored - backend CloseSession will clean up too.
        }
        await this.runAction(async () => CloseSession(tabID), 'Unable to close session');
        await this.ensureActiveSFTPLoaded(true);
    }

    private async loadSFTP(tabID: string, targetPath: string): Promise<void> {
        this.sftpState = { ...this.sftpState, tabId: tabID, loading: true, error: '' };
        this.render();
        try {
            const entries = targetPath ? await NavigateSFTP(tabID, targetPath) : await ListSFTPFiles(tabID, '');
            this.sftpState = {
                ...this.sftpState,
                tabId: tabID,
                path: inferDirectory(entries, targetPath),
                entries,
                loading: false,
                error: '',
                selectedFiles: [],
            };
        } catch (error) {
            this.pushNotification('error', formatError('Unable to load SFTP files', error));
            this.sftpState = {
                ...this.sftpState,
                tabId: tabID,
                loading: false,
                error: formatError('Unable to load SFTP files', error),
            };
        }
        this.render();
    }

    private async openRemoteFile(tabID: string, targetPath: string): Promise<void> {
        this.sftpState = {
            ...this.sftpState,
            tabId: tabID,
            editorOpen: true,
            editorPath: targetPath,
            editorLoading: true,
            editorError: '',
        };
        this.render();
        try {
            const content = await ReadSFTPFile(tabID, targetPath);
            this.sftpState = {
                ...this.sftpState,
                editorOpen: true,
                editorPath: targetPath,
                editorContent: content,
                editorLoading: false,
                editorDirty: false,
                editorError: '',
            };
        } catch (error) {
            this.pushNotification('error', formatError('Unable to read remote file', error));
            this.sftpState = {
                ...this.sftpState,
                editorOpen: true,
                editorLoading: false,
                editorError: formatError('Unable to read remote file', error),
            };
        }
        this.render();
    }

    private async saveRemoteFile(tabID: string): Promise<void> {
        this.sftpState = { ...this.sftpState, editorSaving: true, editorError: '' };
        this.render();
        try {
            await SaveSFTPFile(tabID, this.sftpState.editorPath, this.sftpState.editorContent);
            this.sftpState = { ...this.sftpState, editorSaving: false, editorDirty: false, editorError: '' };
        } catch (error) {
            this.pushNotification('error', formatError('Unable to save remote file', error));
            this.sftpState = {
                ...this.sftpState,
                editorSaving: false,
                editorError: formatError('Unable to save remote file', error),
            };
        }
        this.render();
    }

    private async uploadToSFTP(tabID: string): Promise<void> {
        this.sftpState = { ...this.sftpState, error: '' };
        this.render();
        try {
            const localPaths = await SelectUploadFiles();
            if (!localPaths || localPaths.length === 0) {
                return;
            }
            await UploadSFTPFiles(tabID, this.sftpState.path || '.', localPaths);
            await this.loadSFTP(tabID, this.sftpState.path || '.');
        } catch (error) {
            this.pushNotification('error', formatError('Unable to upload files', error));
            this.sftpState = {
                ...this.sftpState,
                error: formatError('Unable to upload files', error),
            };
            this.render();
        }
    }

    private async downloadFromSFTP(tabID: string): Promise<void> {
        if (this.sftpState.selectedFiles.length === 0) {
            this.sftpState = { ...this.sftpState, error: 'Select at least one file to download.' };
            this.render();
            return;
        }
        try {
            const localDir = await SelectDownloadDirectory();
            if (!localDir) {
                return;
            }
            await DownloadSFTPFiles(tabID, localDir, this.sftpState.selectedFiles);
            this.sftpState = { ...this.sftpState, selectedFiles: [], error: '' };
            this.render();
        } catch (error) {
            this.pushNotification('error', formatError('Unable to download files', error));
            this.sftpState = {
                ...this.sftpState,
                error: formatError('Unable to download files', error),
            };
            this.render();
        }
    }

    private toggleSFTPFileSelection(targetPath: string, checked: boolean): void {
        const selected = new Set(this.sftpState.selectedFiles);
        if (checked) {
            selected.add(targetPath);
        } else {
            selected.delete(targetPath);
        }
        this.sftpState = { ...this.sftpState, selectedFiles: Array.from(selected) };
        this.render();
    }

    private async loadVaultSecrets(targetPath: string): Promise<void> {
        const normalizedPath = targetPath === '.' ? '' : targetPath;
        this.vaultState = { ...this.vaultState, loading: true, error: '' };
        this.render();
        try {
            const entries = await ListVaultSecrets(normalizedPath);
            this.vaultState = {
                path: normalizedPath,
                entries,
                loading: false,
                error: '',
                loaded: true,
            };
        } catch (error) {
            this.pushNotification('error', formatError('Unable to load Vault secrets', error));
            this.vaultState = {
                ...this.vaultState,
                loading: false,
                error: formatError('Unable to load Vault secrets', error),
                loaded: true,
            };
        }
        this.render();
    }

    private attachActiveTerminal(): void {
        const host = document.querySelector<HTMLDivElement>('#terminal-host');
        if (!host) {
            return;
        }
        host.innerHTML = '';

        const activeTab = this.activeTab();
        if (!activeTab) {
            host.innerHTML = '<div class="empty-state">Open an SSH session to start a terminal.</div>';
            return;
        }
        if (activeTab.protocolId !== 'ssh') {
            host.innerHTML = `<div class="empty-state">${escapeHtml(activeTab.protocolId.toUpperCase())} session opened in tab mode. Terminal is available for SSH tabs.</div>`;
            return;
        }

        const terminalState = this.ensureTerminalSubscription(activeTab.id);
        host.appendChild(terminalState.wrapper);
        if (!terminalState.opened) {
            terminalState.terminal.open(terminalState.inner);
            terminalState.fitAddon.fit();
            terminalState.opened = true;
            terminalState.terminal.focus();
            terminalState.terminal.onData((data: string) => {
                void SendSSHInput(activeTab.id, data).catch((error) => {
                    this.setErrorMessage(formatError('Unable to send terminal input', error));
                    this.render();
                });
            });
        }
        this.fitActiveTerminal();
    }

    private ensureTerminalSubscription(tabID: string): TerminalState {
        const existing = this.terminals.get(tabID);
        if (existing) {
            return existing;
        }

        const wrapper = document.createElement('div');
        wrapper.className = 'terminal-pane';
        const inner = document.createElement('div');
        inner.className = 'terminal-instance';
        wrapper.appendChild(inner);

        const terminal = new Terminal({
            cursorBlink: true,
            fontFamily: 'ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace',
            theme: { background: '#06101f', foreground: '#e6edf7' },
            scrollback: 2000,
        });
        const fitAddon = new FitAddon();
        terminal.loadAddon(fitAddon);
        terminal.loadAddon(new WebLinksAddon());
        terminal.writeln('Connecting...');

        const unsubscribe = EventsOn(`terminal:output:${tabID}`, (...payload: unknown[]) => {
            const data = payload[0] as { data?: string } | undefined;
            const chunk = data?.data ?? '';
            terminal.write(chunk);
            if (!chunk) {
                return;
            }
            const current = this.terminalOutputHistory.get(tabID) ?? '';
            const updated = `${current}${chunk}`;
            this.terminalOutputHistory.set(tabID, updated.slice(-12000));
        });

        const terminalState: TerminalState = { terminal, fitAddon, wrapper, inner, opened: false, unsubscribe };
        this.terminals.set(tabID, terminalState);
        return terminalState;
    }

    private fitActiveTerminal(): void {
        const activeTab = this.activeTab();
        if (!activeTab) {
            return;
        }
        const terminalState = this.terminals.get(activeTab.id);
        if (!terminalState || !terminalState.opened) {
            return;
        }
        requestAnimationFrame(() => {
            terminalState.fitAddon.fit();
            const dimensions = terminalState.terminal.cols > 0 && terminalState.terminal.rows > 0;
            if (dimensions) {
                void ResizeTerminal(activeTab.id, terminalState.terminal.cols, terminalState.terminal.rows).catch(() => undefined);
            }
        });
    }

    private scrollChatToBottom(): void {
        const messages = document.querySelector<HTMLDivElement>('#chat-messages');
        if (messages) {
            messages.scrollTop = messages.scrollHeight;
        }
    }

    private renderSessionProfiles(): string {
        if (!this.shellState || this.shellState.sessionProfiles.length === 0) {
            return '<div class="empty-state">No saved sessions yet.</div>';
        }
        const filteredProfiles = this.filteredSessionProfiles();
        if (filteredProfiles.length === 0) {
            return '<div class="empty-state">No sessions for selected tags.</div>';
        }
        return filteredProfiles.map((profile) => `
            <article class="session-card" data-session-item="${escapeHtml(profile.id)}">
                <div>
                    <div class="session-title-row">
                        <strong>${escapeHtml(profile.name)}</strong>
                        <span class="pill small">${escapeHtml(profile.protocolId.toUpperCase())}</span>
                    </div>
                    <div class="session-meta">${escapeHtml(profile.username)}@${escapeHtml(profile.host)}:${escapeHtml(String(profile.port))}</div>
                    <div class="session-tags">${profile.tags.map((tag) => `<span>${escapeHtml(tag)}</span>`).join('')}</div>
                </div>
            </article>
        `).join('');
    }

    private renderSessionTagFilters(): string {
        const tags = this.allSessionTags();
        const hasUntagged = (this.shellState?.sessionProfiles ?? []).some((profile) => (profile.tags ?? []).length === 0);
        const filterKeys = hasUntagged ? [...tags, this.untaggedFilterTag] : tags;
        if (filterKeys.length === 0) {
            return '<div class="section-copy">No tags yet.</div>';
        }
        return `
            <div class="tag-filter-list">
                ${filterKeys.map((tag) => `
                    <button
                        class="tag-filter-item ${this.selectedSessionTags.has(tag) ? 'active' : ''}"
                        data-toggle-session-tag-filter="${escapeHtml(tag)}"
                        type="button"
                    >${tag === this.untaggedFilterTag ? 'Untagged' : escapeHtml(tag)}</button>
                `).join('')}
            </div>
        `;
    }

    private renderTabs(): string {
        if (!this.shellState || this.shellState.activeSessions.length === 0) {
            return '<div class="tab empty">No active sessions</div>';
        }
        return this.shellState.activeSessions.map((tab) => `
            <button class="tab ${tab.id === this.activeTabId ? 'active' : ''}" data-tab-id="${escapeHtml(tab.id)}">
                <span>${escapeHtml(tab.title)}</span>
                <span class="tab-close" data-close-tab="${escapeHtml(tab.id)}">×</span>
            </button>
        `).join('');
    }

    private renderSFTPActions(): string {
        const activeTab = this.activeTab();
        if (!activeTab || activeTab.protocolId !== 'ssh') {
            return '<span class="section-copy">Select an SSH tab</span>';
        }
        const showRefresh = !this.sftpState.loading && (this.sftpState.entries.length > 0 || !!this.sftpState.error);
        const canTransfer = !this.sftpState.loading && !this.sftpState.error && this.sftpState.entries.length > 0;
        return `
            <div class="sftp-actions">
                ${showRefresh ? '<button class="icon-button sftp-action-button" data-refresh-sftp title="Reload current path" aria-label="Reload current path">↻</button>' : ''}
                ${canTransfer ? '<button class="icon-button sftp-action-button" data-sftp-upload title="Upload files" aria-label="Upload files">⤴</button>' : ''}
                ${canTransfer ? `<button class="icon-button sftp-action-button" data-sftp-download ${this.sftpState.selectedFiles.length === 0 ? 'disabled' : ''} title="Download selected files" aria-label="Download selected files">⤵${this.sftpState.selectedFiles.length > 0 ? ` ${this.sftpState.selectedFiles.length}` : ''}</button>` : ''}
            </div>
        `;
    }

    private renderSFTPBrowser(): string {
        const activeTab = this.activeTab();
        if (!activeTab) {
            return '<div class="empty-state">Open a session tab to browse files.</div>';
        }
        if (activeTab.protocolId !== 'ssh') {
            return '<div class="empty-state">SFTP browsing is available for SSH tabs.</div>';
        }
        if (this.sftpState.loading) {
            return '<div class="empty-state">Loading files…</div>';
        }
        if (this.sftpState.error) {
            return `<div class="error-banner compact">${escapeHtml(this.sftpState.error)}</div>`;
        }
        if (this.sftpState.entries.length === 0) {
            return '<div class="empty-state">SFTP opens automatically for the active SSH session.</div>';
        }
        return `
            <div class="sftp-explorer">
                <div class="sftp-path-row">
                    <span class="sftp-path">${escapeHtml(this.sftpState.path || '.')}</span>
                    <button class="icon-button sftp-action-button" data-sftp-up title="Go to parent directory" aria-label="Go to parent directory">↑</button>
                </div>
                <div class="sftp-list">
                    <div class="sftp-list-header">
                        <span></span>
                        <span>Name</span>
                        <span>Size</span>
                        <span>Mode</span>
                        <span>Modified</span>
                        <span></span>
                    </div>
                    ${this.sftpState.entries.map((entry) => entry.isDir
                        ? `<button class="sftp-row sftp-row-button sftp-dir" data-sftp-dir="${escapeHtml(entry.path)}">
                            <span></span>
                            <span class="sftp-row-name"><span class="sftp-entry-icon" aria-hidden="true">📁</span><span>${escapeHtml(entry.name)}</span></span>
                            <span>—</span>
                            <span>${escapeHtml(entry.mode || '—')}</span>
                            <span>${escapeHtml(entry.modTime || '—')}</span>
                            <span></span>
                        </button>`
                        : `<div class="sftp-row sftp-file ${this.sftpState.selectedFiles.includes(entry.path) ? 'selected' : ''}">
                            <span class="sftp-row-check"><input type="checkbox" data-sftp-select-file="${escapeHtml(entry.path)}" ${this.sftpState.selectedFiles.includes(entry.path) ? 'checked' : ''} /></span>
                            <span class="sftp-row-name"><span class="sftp-entry-icon" aria-hidden="true">📄</span><span>${escapeHtml(entry.name)}</span></span>
                            <span>${escapeHtml(formatBytes(entry.size))}</span>
                            <span>${escapeHtml(entry.mode || '—')}</span>
                            <span>${escapeHtml(entry.modTime || '—')}</span>
                            <span><button class="action-button secondary sftp-inline-button" data-sftp-file-edit="${escapeHtml(entry.path)}">Edit</button></span>
                        </div>`).join('')}
                </div>
            </div>
        `;
    }

    private renderRemoteEditorModal(): string {
        if (!this.sftpState.editorOpen) {
            return '';
        }
        return `
            <div class="remote-editor">
                <div class="modal-overlay">
                    <div class="modal-dialog wide remote-editor-dialog">
                        <div class="panel-header compact-header">
                            <div>
                                <div class="eyebrow">SFTP editor</div>
                                <h2>${escapeHtml(this.sftpState.editorPath || 'Remote file')}</h2>
                            </div>
                            <div class="section-actions">
                                <button class="action-button secondary" data-close-remote-editor>Close</button>
                                <button class="action-button secondary" data-save-remote-file ${this.sftpState.editorSaving || this.sftpState.editorLoading ? 'disabled' : ''}>${this.sftpState.editorSaving ? 'Saving…' : 'Save'}</button>
                            </div>
                        </div>
                        <div class="modal-body remote-editor-body">
                            ${this.sftpState.editorLoading ? '<div class="empty-state">Loading file…</div>' : `
                                ${this.sftpState.editorError ? `<div class="error-banner compact">${escapeHtml(this.sftpState.editorError)}</div>` : ''}
                                ${this.sftpState.editorError && !this.sftpState.editorContent
                                    ? ''
                                    : `<textarea class="remote-editor-input" data-remote-editor>${escapeHtml(this.sftpState.editorContent)}</textarea>`}
                            `}
                        </div>
                    </div>
                </div>
            </div>
        `;
    }

    private renderVaultBrowser(): string {
        if (this.vaultState.loading) {
            return '<div class="empty-state">Loading secrets…</div>';
        }
        if (this.vaultState.error) {
            return '<div class="error-banner compact">Unable to load secrets. Check notifications.</div>';
        }
        if (!this.vaultState.loaded) {
            return '<div class="empty-state">Open Vault browser to load secrets.</div>';
        }
        const entriesBlock = this.vaultState.entries.length > 0
            ? this.vaultState.entries.map((entry) => entry.isDir
                ? `<button class="sftp-entry sftp-dir" data-vault-dir="${escapeHtml(entry.path)}"><span>${escapeHtml(entry.name)}</span><small>dir</small></button>`
                : `<div class="sftp-entry sftp-file"><span>${escapeHtml(entry.name)}</span><small>secret</small></div>`).join('')
            : '<div class="empty-state">No secrets in this path.</div>';
        return `
            <div class="sftp-path-row">
                <span class="sftp-path">${escapeHtml(this.vaultState.path || '/')}</span>
                <button class="action-button secondary" data-vault-up>..</button>
            </div>
            <div class="sftp-list">
                ${entriesBlock}
            </div>
        `;
    }

    private async loadCloudModels(force: boolean): Promise<void> {
        const provider = this.selectedProvider();
        if (!provider) {
            return;
        }
        const endpoint = (this.cloudDraftEndpoint || provider.endpoint || '').trim();
        const token = (this.cloudDraftToken || '').trim();
        if (!endpoint) {
            this.cloudModels = [];
            this.cloudModelsEndpoint = '';
            this.cloudModelsError = 'Enter endpoint first.';
            this.render();
            return;
        }
        if (!force && endpoint === this.cloudModelsEndpoint && this.cloudModels.length > 0) {
            return;
        }
        this.cloudModelsLoading = true;
        this.cloudModelsError = '';
        this.render();
        try {
            const models = await ListCloudModels(endpoint, token);
            this.cloudModels = models;
            this.cloudModelsEndpoint = endpoint;
            this.cloudModelsError = models.length > 0 ? '' : 'No models returned by API.';
        } catch (error) {
            this.cloudModels = [];
            this.cloudModelsEndpoint = endpoint;
            this.cloudModelsError = formatError('Unable to load models', error);
            this.pushNotification('error', this.cloudModelsError);
        } finally {
            this.cloudModelsLoading = false;
            this.render();
        }
    }

    private renderProviderSetup(): string {
        const provider = this.selectedProvider();
        if (!provider) {
            return '<div class="empty-state">No AI provider configured yet.</div>';
        }
        const selectedModel = this.cloudDraftModel || provider.model || '';
        return `
            <form class="provider-form" data-cloud-form>
                <label>
                    <span>Model</span>
                    <input name="model" data-cloud-model type="text" value="${escapeHtml(selectedModel)}" list="cloud-model-suggestions" placeholder="gpt-5.6" required />
                    <datalist id="cloud-model-suggestions">
                        ${this.cloudModels.map((model) => `<option value="${escapeHtml(model)}"></option>`).join('')}
                    </datalist>
                </label>
                <label>
                    <span>Endpoint</span>
                    <input type="url" data-cloud-endpoint name="endpoint" value="${escapeHtml(this.cloudDraftEndpoint || provider.endpoint || '')}" placeholder="https://api.example.com/v1" required />
                </label>
                <label>
                    <span>API token (optional)</span>
                    <input type="password" data-cloud-token name="token" value="${escapeHtml(this.cloudDraftToken)}" placeholder="sk-..." />
                </label>
                <div class="provider-form-actions">
                    <button class="action-button secondary" type="button" data-load-cloud-models ${this.cloudModelsLoading ? 'disabled' : ''}>${this.cloudModelsLoading ? 'Loading models…' : 'Load models'}</button>
                    ${this.cloudModelsError ? `<span class="section-copy">${escapeHtml(this.cloudModelsError)}</span>` : ''}
                </div>
                <div class="provider-form-actions">
                    <button class="action-button" type="submit">Save cloud provider</button>
                </div>
            </form>
        `;
    }

    private renderPortForwardRules(): string {
        const rules: PortForwardRule[] = this.shellState?.settings.portForwardRules ?? [];
        const sshProfiles = (this.shellState?.sessionProfiles ?? []).filter((p) => p.protocolId === 'ssh');
        const profileName = (hostId: string) => {
            const p = sshProfiles.find((s) => s.id === hostId);
            return p ? `${p.name} (${p.host})` : hostId;
        };
        if (rules.length === 0) {
            return '<div class="empty-state" style="padding:0.75rem 0;">No rules yet. Add one below.</div>';
        }
        return `
            <div class="pf-rules-list">
                ${rules.map((rule, idx) => `
                    <div class="pf-rule ${rule.enabled ? 'pf-rule-enabled' : 'pf-rule-disabled'}">
                        <span class="pf-rule-ports">${escapeHtml(rule.localPort || rule.ports || '')}</span>
                        <span class="pf-rule-arrow">→</span>
                        <span class="pf-rule-host">${escapeHtml(rule.remoteHost)}:${escapeHtml(rule.remotePort)} via ${escapeHtml(profileName(rule.hostId))}</span>
                        <span class="pf-rule-spacer"></span>
                        <button class="action-button secondary" data-pf-start-stop="${idx}" title="${rule.enabled ? 'Stop' : 'Start'}">${rule.enabled ? 'Stop' : 'Start'}</button>
                        <button class="icon-button danger" data-pf-delete="${idx}" title="Delete rule">✕</button>
                    </div>
                `).join('')}
            </div>
        `;
    }

    private renderMessages(): string {
        if (!this.shellState) {
            return '';
        }
        if (!this.hasConfiguredProvider()) {
            return '<div class="empty-state">Configure a provider in Settings to start chatting.</div>';
        }
        return (this.shellState.ai.messages ?? []).map((message) => `
            <div class="message ${escapeClassName(message.role)}">${message.role === 'assistant' ? renderMarkdown(message.content) : escapeHtml(message.content)}</div>
        `).join('');
    }

    private renderSettingsModal(): string {
        const tabs: Array<{ id: SettingsTab; label: string }> = [
            { id: 'ai', label: 'AI' },
            { id: 'vault', label: 'Vault' },
            { id: 'sshconfig', label: 'SSH Config' },
            { id: 'portforward', label: 'Port forwarding' },
            { id: 'theme', label: 'Theme' },
        ];
        const sshProfiles = (this.shellState?.sessionProfiles ?? []).filter((profile) => profile.protocolId === 'ssh');
        return `
            <div class="modal-overlay">
                <div class="modal-dialog wide settings-dialog">
                    <div class="panel-header compact-header">
                        <div>
                            <div class="eyebrow">Settings</div>
                            <h2>Preferences</h2>
                        </div>
                        <button class="icon-button" data-close-modal>×</button>
                    </div>
                    <nav class="modal-tabs">
                        ${tabs.map((t) => `<button class="modal-tab ${this.settingsTab === t.id ? 'active' : ''}" data-settings-tab="${t.id}">${t.label}</button>`).join('')}
                    </nav>
                    <div class="modal-body">
                        <div class="modal-tab-panel ${this.settingsTab === 'ai' ? 'active' : ''}">
                            <div class="section-title">AI Provider</div>
                            ${this.renderProviderSetup()}
                        </div>
                        <div class="modal-tab-panel ${this.settingsTab === 'vault' ? 'active' : ''}">
                            <div class="section-title">Vault Browser</div>
                            <form class="provider-form" data-vault-settings-form>
                                <label>
                                    <span>Vault instance URL</span>
                                    <input type="url" data-vault-address name="vaultAddress" value="${escapeHtml(this.vaultDraftAddress)}" placeholder="https://vault.example.com" />
                                </label>
                                <label>
                                    <span>Mountpoint</span>
                                    <input type="text" data-vault-mount name="vaultMountPoint" value="${escapeHtml(this.vaultDraftMountPoint)}" placeholder="secret" />
                                </label>
                                <label>
                                    <span>Secret source</span>
                                    <select name="vaultProvider" data-vault-provider>
                                        <option value="vault" ${this.vaultDraftProvider === 'vault' ? 'selected' : ''}>HashiCorp Vault</option>
                                        <option value="keepass" ${this.vaultDraftProvider === 'keepass' ? 'selected' : ''}>KeePass</option>
                                    </select>
                                </label>
                                ${this.vaultDraftProvider === 'vault' ? `
                                <label>
                                    <span>Token</span>
                                    <input type="password" data-vault-token name="vaultToken" value="${escapeHtml(this.vaultDraftToken)}" placeholder="hvs...." />
                                </label>
                                <label class="inline-check">
                                    <span>Auto-renew token</span>
                                    <input data-vault-renew name="vaultAutoRenewToken" type="checkbox" ${this.vaultDraftAutoRenewToken ? 'checked' : ''} />
                                </label>
                                ` : `
                                <label>
                                    <span>KeePass database path</span>
                                    <input type="text" data-keepass-db-path name="keepassDatabasePath" value="${escapeHtml(this.vaultDraftKeePassDatabasePath)}" placeholder="~/.config/KeePass/database.kdbx" />
                                </label>
                                <label>
                                    <span>KeePass password</span>
                                    <input type="password" data-keepass-password name="keepassPassword" value="${escapeHtml(this.vaultDraftKeePassPassword)}" />
                                </label>
                                `}
                                <div class="provider-form-actions">
                                    <button class="action-button" type="submit">Save Vault settings</button>
                                </div>
                            </form>
                        </div>
                        <div class="modal-tab-panel ${this.settingsTab === 'sshconfig' ? 'active' : ''}">
                            <div class="section-title">Import SSH Config</div>
                            <div class="import-box">
                                <label class="import-label" for="ssh-config-import">Paste SSH config block</label>
                                <textarea id="ssh-config-import" data-ssh-config-import placeholder="Host prod&#10;  HostName prod.internal&#10;  User ops&#10;  ProxyJump bastion">${escapeHtml(this.sshConfigDraft)}</textarea>
                                <button class="action-button secondary" data-import-ssh-config>Import</button>
                            </div>
                        </div>
                        <div class="modal-tab-panel ${this.settingsTab === 'portforward' ? 'active' : ''}">
                            <div class="section-title">Port forwarding rules</div>
                            ${this.renderPortForwardRules()}
                            <form class="provider-form" data-pf-add-form style="margin-top:0.5rem;">
                                <div style="display:grid;grid-template-columns:1fr 1fr 160px 1fr auto;gap:0.5rem;align-items:end;">
                                    <label style="margin:0;"><span style="font-size:0.78rem;">Local port(s)</span><input type="text" name="pfLocalPort" value="${escapeHtml(this.pfNewLocalPort)}" placeholder="8080,9000-9005" /></label>
                                    <label style="margin:0;"><span style="font-size:0.78rem;">Remote host</span><input type="text" name="pfRemoteHost" value="${escapeHtml(this.pfNewRemoteHost)}" placeholder="db.internal" /></label>
                                    <label style="margin:0;"><span style="font-size:0.78rem;">Remote port</span><input type="number" name="pfRemotePort" min="1" max="65535" value="${escapeHtml(this.pfNewRemotePort)}" placeholder="5432" /></label>
                                    <label style="margin:0;"><span style="font-size:0.78rem;">Target SSH host</span>
                                        <select name="pfHostId">
                                            <option value="">Select host</option>
                                            ${sshProfiles.map((profile) => `<option value="${escapeHtml(profile.id)}" ${this.pfNewHostId === profile.id ? 'selected' : ''}>${escapeHtml(profile.name)} (${escapeHtml(profile.host)})</option>`).join('')}
                                        </select>
                                    </label>
                                    <button class="action-button" type="submit" style="align-self:flex-end;">Add rule</button>
                                </div>
                            </form>
                        </div>
                        <div class="modal-tab-panel ${this.settingsTab === 'theme' ? 'active' : ''}">
                            <div class="section-title">Appearance</div>
                            <div class="theme-toggle-row">
                                <span>Theme</span>
                                <div class="theme-switch">
                                    <button class="${this.theme === 'dark' ? 'active' : ''}" data-set-theme="dark">🌙 Dark</button>
                                    <button class="${this.theme === 'light' ? 'active' : ''}" data-set-theme="light">☀ Light</button>
                                    <button class="${this.theme === 'green' ? 'active' : ''}" data-set-theme="green">🟢 Green</button>
                                </div>
                            </div>
                        </div>
                    </div>
                </div>
            </div>
        `;
    }

    private openSessionModalForCreate(): void {
        this.editingProfileID = '';
        this.sessionForm = this.defaultSessionForm();
        this.sessionTagDraft = '';
        this.sessionTagInputVisible = false;
        this.sessionNameAuto = true;
        this.sessionModalTab = 'host';
        this.showSessionModal = true;
        this.render();
    }

    private openSessionModalForEdit(profileID: string): void {
        const profile = this.shellState?.sessionProfiles.find((entry) => entry.id === profileID);
        if (!profile) {
            return;
        }
        this.editingProfileID = profileID;
        this.sessionForm = this.sessionFormFromProfile(profile);
        this.sessionTagDraft = '';
        this.sessionTagInputVisible = false;
        this.sessionNameAuto = false;
        this.sessionModalTab = 'host';
        this.showSessionModal = true;
        this.render();
    }

    private sessionFormFromProfile(profile: sessions.Profile): SessionFormState {
        const options = profile.options ?? {};
        const authMethod = options.auth_method === 'key' && this.supportsKeyAuth(profile.protocolId) ? 'key' : 'password';
        return {
            name: profile.name || profile.host || '',
            host: profile.host || '',
            port: String(profile.port || (profile.protocolId === 'rdp' ? 3389 : 22)),
            username: profile.username || '',
            password: '',
            authMethod,
            privateKeyPath: options.ssh_private_key_path ?? '',
            protocolId: profile.protocolId || 'ssh',
            tags: Array.isArray(profile.tags) ? [...profile.tags] : [],
            proxyJump: options.proxy_jump ?? '',
            localForwards: options.local_forwards ?? '',
            useSSHAgent: options.use_ssh_agent === 'true',
        };
    }

    private renderSessionModal(): string {
        const tabs: Array<{ id: SessionModalTab; label: string }> = [
            { id: 'host', label: 'Host' },
            { id: 'auth', label: 'Authorization' },
            { id: 'network', label: 'Network' },
            { id: 'other', label: 'Other' },
        ];
        const supportsKeyAuth = this.supportsKeyAuth(this.sessionForm.protocolId);
        const supportsSSHAdvancedOptions = this.supportsSSHAdvancedOptions(this.sessionForm.protocolId);
        const isEditing = this.editingProfileID !== '';
        return `
            <div class="modal-overlay">
                <div class="modal-dialog">
                    <div class="panel-header compact-header">
                        <div>
                            <div class="eyebrow">${isEditing ? 'Session settings' : 'New session'}</div>
                            <h2>${isEditing ? 'Edit session profile' : 'Create session profile'}</h2>
                        </div>
                        <button class="icon-button" data-close-modal>×</button>
                    </div>
                    <nav class="modal-tabs">
                        ${tabs.map((t) => `<button class="modal-tab ${this.sessionModalTab === t.id ? 'active' : ''}" data-session-modal-tab="${t.id}">${t.label}</button>`).join('')}
                    </nav>
                    <div class="modal-body">
                        <form class="session-form" data-session-form>
                            <div class="modal-tab-panel ${this.sessionModalTab === 'host' ? 'active' : ''}">
                                <label><span>Host</span><input name="host" value="${escapeHtml(this.sessionForm.host)}" required /></label>
                                <label><span>Port</span><input name="port" type="number" value="${escapeHtml(this.sessionForm.port)}" min="1" required /></label>
                                <label>
                                    <span>Protocol</span>
                                    <select name="protocolId">
                                        <option value="ssh" ${this.sessionForm.protocolId === 'ssh' ? 'selected' : ''}>SSH</option>
                                        <option value="sftp" ${this.sessionForm.protocolId === 'sftp' ? 'selected' : ''}>SFTP</option>
                                        <option value="rdp" ${this.sessionForm.protocolId === 'rdp' ? 'selected' : ''}>RDP</option>
                                    </select>
                                </label>
                            </div>
                            <div class="modal-tab-panel ${this.sessionModalTab === 'auth' ? 'active' : ''}">
                                <label><span>Username</span><input name="username" value="${escapeHtml(this.sessionForm.username)}" required /></label>
                                ${supportsKeyAuth ? `
                                <label>
                                    <span>Auth method</span>
                                    <select name="authMethod">
                                        <option value="password" ${this.sessionForm.authMethod === 'password' ? 'selected' : ''}>Password</option>
                                        <option value="key" ${this.sessionForm.authMethod === 'key' ? 'selected' : ''}>SSH key</option>
                                    </select>
                                </label>
                                ` : ''}
                                ${this.sessionForm.authMethod === 'key' && supportsKeyAuth
                                    ? `<label><span>Private key path</span><input name="privateKeyPath" value="${escapeHtml(this.sessionForm.privateKeyPath)}" placeholder="~/.ssh/id_ed25519" required /></label>`
                                    : `<label><span>Password</span><input name="password" type="password" value="${escapeHtml(this.sessionForm.password)}" /></label>`}
                            </div>
                            <div class="modal-tab-panel ${this.sessionModalTab === 'other' ? 'active' : ''}">
                                <label><span>Name</span><input name="name" value="${escapeHtml(this.sessionForm.name)}" required /></label>
                                <label>
                                    <span>Tags</span>
                                    <div class="session-tag-editor">
                                        <div class="session-tag-list">
                                            ${this.sessionForm.tags.map((tag) => `
                                                <span class="session-tag-chip">
                                                    ${escapeHtml(tag)}
                                                    <button type="button" class="session-tag-chip-remove" data-session-tag-remove="${escapeHtml(tag)}" aria-label="Remove tag ${escapeHtml(tag)}">×</button>
                                                </span>
                                            `).join('')}
                                        </div>
                                        <div class="session-tag-input-row">
                                            ${this.sessionTagInputVisible
            ? `<input data-session-tag-input placeholder="New tag" value="${escapeHtml(this.sessionTagDraft)}" />`
            : '<button type="button" class="action-button secondary" data-session-tag-add-open aria-label="Add tag">+ Add tag</button>'}
                                        </div>
                                    </div>
                                </label>
                                ${supportsSSHAdvancedOptions ? `
                                <label class="inline-check"><span>Use SSH agent</span><input name="useSSHAgent" type="checkbox" ${this.sessionForm.useSSHAgent ? 'checked' : ''} /></label>
                                ` : ''}
                            </div>
                            <div class="modal-tab-panel ${this.sessionModalTab === 'network' ? 'active' : ''}">
                                ${supportsSSHAdvancedOptions ? `
                                <label><span>ProxyJump</span><input name="proxyJump" value="${escapeHtml(this.sessionForm.proxyJump)}" placeholder="bastion or user@bastion:22" /></label>
                                <label><span>Local tunnels</span><input name="localForwards" value="${escapeHtml(this.sessionForm.localForwards)}" placeholder="15432:db.internal:5432,18080:127.0.0.1:8080" /></label>
                                ` : '<div class="empty-state">Network settings are available only for SSH and SFTP sessions.</div>'}
                            </div>
                            <div style="padding: 0 1.5rem 1.25rem; display:flex; gap:0.75rem; justify-content:flex-end;">
                                <button type="button" class="action-button secondary" data-close-modal>Cancel</button>
                                <button type="submit" class="action-button">${isEditing ? 'Update' : 'Save'}</button>
                            </div>
                        </form>
                    </div>
                </div>
            </div>
        `;
    }

    private renderSessionContextMenu(): string {
        if (!this.sessionContextMenu.visible) {
            return '';
        }
        return `
            <div class="session-context-overlay" data-session-context-overlay>
                <div class="session-context-menu" style="left:${this.sessionContextMenu.x}px;top:${this.sessionContextMenu.y}px;">
                    <button class="session-context-item" data-session-context-open="${escapeHtml(this.sessionContextMenu.profileId)}">Open</button>
                    <button class="session-context-item" data-session-context-edit="${escapeHtml(this.sessionContextMenu.profileId)}">Edit settings</button>
                    <button class="session-context-item danger" data-session-context-delete="${escapeHtml(this.sessionContextMenu.profileId)}">Delete</button>
                </div>
            </div>
        `;
    }

    private hideSessionContextMenu(): void {
        if (!this.sessionContextMenu.visible) {
            return;
        }
        this.sessionContextMenu = { visible: false, x: 0, y: 0, profileId: '' };
        this.render();
    }

    private renderHostKeyDialog(): string {
        const { hostname, fingerprint } = this.hostKeyDialog;
        return `
            <div class="modal-overlay">
                <div class="modal-dialog">
                    <div class="panel-header compact-header">
                        <div>
                            <div class="eyebrow">Security alert</div>
                            <h2>Unknown host key</h2>
                        </div>
                    </div>
                    <div class="modal-body">
                        <p>The authenticity of host <strong>${escapeHtml(hostname)}</strong> cannot be established.</p>
                        <p>Key fingerprint:<br><code>${escapeHtml(fingerprint)}</code></p>
                        <p>Do you want to trust this host and add it to your known_hosts file?</p>
                        <div style="display:flex;gap:0.75rem;justify-content:flex-end;padding-top:1rem;">
                            <button class="action-button secondary" data-reject-host-key>Reject</button>
                            <button class="action-button" data-accept-host-key>Trust &amp; Connect</button>
                        </div>
                    </div>
                </div>
            </div>
        `;
    }

    private withLatestTerminalOutput(message: string): string {
        const output = this.latestActiveTerminalOutput();
        if (!output) {
            return message;
        }
        return `${message}\n\n[Latest console output]\n${output}`;
    }

    private latestActiveTerminalOutput(): string {
        const activeTab = this.activeTab();
        if (!activeTab) {
            return '';
        }
        const text = this.terminalOutputHistory.get(activeTab.id) ?? '';
        if (!text.trim()) {
            return '';
        }
        const normalized = text.replace(/\r/g, '').replace(/\u001b\[[0-9;?]*[a-zA-Z]/g, '');
        const lines = normalized.split('\n').map((line) => line.trimEnd()).filter((line) => line.trim() !== '');
        if (lines.length === 0) {
            return '';
        }
        return lines.slice(-24).join('\n');
    }

    private pickActiveTabID(preferredID: string): string {
        const tabs = this.shellState?.activeSessions ?? [];
        if (tabs.some((tab) => tab.id === preferredID)) {
            return preferredID;
        }
        return tabs[0]?.id ?? '';
    }

    private availableSessionInnerTabs(protocolId: string): SessionInnerTab[] {
        if (protocolId === 'rdp') {
            return ['screen'];
        }
        return ['console', 'sftp'];
    }

    private defaultSessionInnerTab(protocolId: string): SessionInnerTab {
        return this.availableSessionInnerTabs(protocolId)[0] ?? 'console';
    }

    private normalizeSessionInnerTab(protocolId: string, current: SessionInnerTab): SessionInnerTab {
        const availableTabs = this.availableSessionInnerTabs(protocolId);
        return availableTabs.includes(current) ? current : this.defaultSessionInnerTab(protocolId);
    }

    private renderSessionInnerTabs(): string {
        const activeTab = this.activeTab();
        if (!activeTab) {
            return '';
        }
        return this.availableSessionInnerTabs(activeTab.protocolId)
            .map((tabID) => `<button class="session-inner-tab ${this.sessionInnerTab === tabID ? 'active' : ''}" data-session-inner-tab="${tabID}">${this.sessionInnerTabLabel(tabID)}</button>`)
            .join('');
    }

    private sessionInnerTabLabel(tab: SessionInnerTab): string {
        if (tab === 'screen') {
            return 'Screen';
        }
        return tab === 'sftp' ? 'SFTP' : 'Console';
    }

    private activeTab(): RuntimeSession | null {
        return this.shellState?.activeSessions.find((tab) => tab.id === this.activeTabId) ?? null;
    }

    private selectedProvider(): AIProvider | null {
        return this.shellState?.ai.providers.find((provider) => provider.selected) ?? null;
    }

    private hasConfiguredProvider(): boolean {
        return (this.shellState?.ai.providers ?? []).some((provider) => provider.configured);
    }

    private async ensureActiveSFTPLoaded(force = false): Promise<void> {
        if (this.sessionInnerTab !== 'sftp') {
            return;
        }
        const activeTab = this.activeTab();
        if (!activeTab || activeTab.protocolId !== 'ssh') {
            return;
        }
        const sameTab = this.sftpState.tabId === activeTab.id;
        if (!force && sameTab && (this.sftpState.loading || this.sftpState.entries.length > 0 || this.sftpState.error)) {
            return;
        }
        const targetPath = sameTab && this.sftpState.path ? this.sftpState.path : '';
        await this.loadSFTP(activeTab.id, targetPath);
    }

    private syncSessionFormFromDOM(source?: EventTarget | null): void {
        const hostValue = root?.querySelector<HTMLInputElement>('input[name="host"]')?.value ?? this.sessionForm.host;
        const nameInput = root?.querySelector<HTMLInputElement>('input[name="name"]');
        const nameValue = nameInput?.value ?? this.sessionForm.name;
        if (source instanceof HTMLInputElement && source.name === 'name') {
            this.sessionNameAuto = !nameValue.trim();
        }
        this.sessionForm.host = hostValue;
        if (this.sessionNameAuto) {
            this.sessionForm.name = hostValue;
        } else {
            this.sessionForm.name = nameValue;
        }
        this.sessionForm.port = root?.querySelector<HTMLInputElement>('input[name="port"]')?.value ?? this.sessionForm.port;
        this.sessionForm.username = root?.querySelector<HTMLInputElement>('input[name="username"]')?.value ?? this.sessionForm.username;
        this.sessionForm.password = root?.querySelector<HTMLInputElement>('input[name="password"]')?.value ?? this.sessionForm.password;
        this.sessionForm.privateKeyPath = root?.querySelector<HTMLInputElement>('input[name="privateKeyPath"]')?.value ?? this.sessionForm.privateKeyPath;
        const authMethodValue = root?.querySelector<HTMLSelectElement>('select[name="authMethod"]')?.value;
        this.sessionForm.authMethod = authMethodValue === 'key'
            ? 'key'
            : (authMethodValue === 'password' ? 'password' : this.sessionForm.authMethod);
        this.sessionForm.protocolId = root?.querySelector<HTMLSelectElement>('select[name="protocolId"]')?.value ?? this.sessionForm.protocolId;
        this.sessionForm.proxyJump = root?.querySelector<HTMLInputElement>('input[name="proxyJump"]')?.value ?? this.sessionForm.proxyJump;
        this.sessionForm.localForwards = root?.querySelector<HTMLInputElement>('input[name="localForwards"]')?.value ?? this.sessionForm.localForwards;
        this.sessionForm.useSSHAgent = root?.querySelector<HTMLInputElement>('input[name="useSSHAgent"]')?.checked ?? this.sessionForm.useSSHAgent;
    }

    private supportsKeyAuth(protocolId: string): boolean {
        return protocolId !== 'rdp';
    }

    private supportsSSHAdvancedOptions(protocolId: string): boolean {
        return protocolId === 'ssh' || protocolId === 'sftp';
    }

    private normalizeNotificationLevel(value?: string): NotificationLevel {
        if (value === 'error' || value === 'warn' || value === 'debug') {
            return value;
        }
        return 'info';
    }

    private pushNotification(level: NotificationLevel, message: string, time = new Date().toISOString()): void {
        const item: NotificationItem = {
            id: `notification-${Date.now()}-${Math.random().toString(16).slice(2, 8)}`,
            level,
            message,
            time,
        };
        this.notifications = [item, ...this.notifications];
        this.toastQueue = [...this.toastQueue, item];
        window.setTimeout(() => {
            this.toastQueue = this.toastQueue.filter((entry) => entry.id !== item.id);
            this.render();
        }, 4500);
        this.render();
    }

    private setErrorMessage(message: string): void {
        this.errorMessage = message;
        if (message.trim()) {
            this.pushNotification('error', message);
        }
    }

    private initializeSettingsDrafts(): void {
        const provider = this.selectedProvider();
        const shellSettings = this.shellState?.settings;
        this.cloudDraftModel = provider?.model ?? '';
        this.cloudDraftEndpoint = provider?.endpoint ?? '';
        this.cloudDraftToken = '';
        this.vaultDraftAddress = shellSettings?.vaultAddress ?? '';
        this.vaultDraftMountPoint = shellSettings?.vaultMountPoint ?? 'secret';
        this.vaultDraftToken = '';
        this.vaultDraftAutoRenewToken = shellSettings?.vaultAutoRenewToken ?? false;
        this.vaultDraftProvider = shellSettings?.vaultProvider ?? 'vault';
        this.vaultDraftKeePassDatabasePath = shellSettings?.keepassDatabasePath ?? '';
        this.vaultDraftKeePassPassword = '';
    }

    private renderToasts(): string {
        if (this.toastQueue.length === 0) {
            return '';
        }
        return `
            <div class="toast-stack">
                ${this.toastQueue.map((item) => `
                    <div class="toast toast-${item.level}">
                        <div class="toast-message">${escapeHtml(item.message)}</div>
                        <div class="toast-time">${escapeHtml(new Date(item.time).toLocaleTimeString())}</div>
                    </div>
                `).join('')}
            </div>
        `;
    }

    private renderNotificationCenter(): string {
        const rows = this.notifications.length > 0
            ? this.notifications.map((item) => `
                <div class="notification-row notification-${item.level}">
                    <div class="notification-meta">
                        <span class="notification-level">${escapeHtml(item.level.toUpperCase())}</span>
                        <span class="notification-time">${escapeHtml(new Date(item.time).toLocaleString())}</span>
                    </div>
                    <div class="notification-message">${escapeHtml(item.message)}</div>
                    <button class="icon-button danger" data-delete-notification="${escapeHtml(item.id)}" title="Delete notification">✕</button>
                </div>
            `).join('')
            : '<div class="empty-state">No notifications yet.</div>';
        return `
            <div class="modal-overlay">
                <div class="modal-dialog wide notification-dialog">
                    <div class="panel-header compact-header">
                        <div>
                            <div class="eyebrow">Notifications</div>
                            <h2>Notification center</h2>
                        </div>
                        <div class="section-actions">
                            <button class="action-button secondary" data-clear-notifications ${this.notifications.length === 0 ? 'disabled' : ''}>Clear all</button>
                            <button class="icon-button" data-close-modal>×</button>
                        </div>
                    </div>
                    <div class="modal-body notification-body">
                        <div class="notification-list">${rows}</div>
                    </div>
                </div>
            </div>
        `;
    }

    private async runAction(action: () => Promise<void>, prefix: string): Promise<void> {
        try {
            await action();
            await this.refresh('');
        } catch (error) {
            this.setErrorMessage(formatError(prefix, error));
            this.render();
        }
    }

    private defaultSessionForm(): SessionFormState {
        return {
            name: '',
            host: '',
            port: '22',
            username: '',
            password: '',
            authMethod: 'password',
            privateKeyPath: '',
            protocolId: 'ssh',
            tags: [],
            proxyJump: '',
            localForwards: '',
            useSSHAgent: false,
        };
    }

    private defaultSFTPState(tabID: string | null): SFTPState {
        return {
            tabId: tabID,
            path: '',
            entries: [],
            loading: false,
            error: '',
            editorOpen: false,
            editorPath: '',
            editorContent: '',
            editorLoading: false,
            editorSaving: false,
            editorDirty: false,
            editorError: '',
            selectedFiles: [],
        };
    }

    private renderScreenWorkspace(): string {
        const activeTab = this.activeTab();
        if (!activeTab) {
            return '<div class="empty-state">Open an RDP session to view the remote screen.</div>';
        }
        if (activeTab.protocolId !== 'rdp') {
            return '<div class="empty-state">Screen view is available only for RDP sessions.</div>';
        }
        const profile = this.shellState?.sessionProfiles.find((entry) => entry.id === activeTab.profileId);
        return `
            <div class="screen-panel">
                <div class="screen-card">
                    <div class="eyebrow">RDP session</div>
                    <h2>${escapeHtml(activeTab.title)}</h2>
                    <p class="screen-copy">Screen view is opened automatically for this RDP session.</p>
                    <div class="screen-meta">
                        <span>${escapeHtml(profile?.username || 'user')}@${escapeHtml(profile?.host || activeTab.title)}:${escapeHtml(String(profile?.port ?? 3389))}</span>
                        <span>Status: ${escapeHtml(activeTab.status)}</span>
                    </div>
                </div>
            </div>
        `;
    }

    private allSessionTags(): string[] {
        const tags = new Set<string>();
        for (const profile of this.shellState?.sessionProfiles ?? []) {
            for (const tag of profile.tags ?? []) {
                const normalized = String(tag ?? '').trim();
                if (normalized) {
                    tags.add(normalized);
                }
            }
        }
        return [...tags].sort((left, right) => left.localeCompare(right));
    }

    private reconcileSelectedSessionTags(): void {
        const allTags = this.allSessionTags();
        const hasUntagged = (this.shellState?.sessionProfiles ?? []).some((profile) => (profile.tags ?? []).length === 0);
        const filterKeys = hasUntagged ? [...allTags, this.untaggedFilterTag] : allTags;
        if (filterKeys.length === 0) {
            this.selectedSessionTags = new Set<string>();
            this.knownSessionTags = new Set<string>();
            this.sessionTagFilterInitialized = false;
            return;
        }
        if (!this.sessionTagFilterInitialized) {
            this.selectedSessionTags = new Set(filterKeys);
            this.knownSessionTags = new Set(filterKeys);
            this.sessionTagFilterInitialized = true;
            return;
        }
        const next = new Set<string>();
        for (const tag of filterKeys) {
            if (this.selectedSessionTags.has(tag)) {
                next.add(tag);
            }
        }
        for (const tag of filterKeys) {
            if (!this.knownSessionTags.has(tag)) {
                next.add(tag);
            }
        }
        this.selectedSessionTags = next;
        this.knownSessionTags = new Set(filterKeys);
    }

    private toggleSessionTagFilter(tag: string): void {
        if (this.selectedSessionTags.has(tag)) {
            this.selectedSessionTags.delete(tag);
        } else {
            this.selectedSessionTags.add(tag);
        }
        this.render();
    }

    private filteredSessionProfiles(): sessions.Profile[] {
        const profiles = this.shellState?.sessionProfiles ?? [];
        const allTags = this.allSessionTags();
        const hasUntagged = profiles.some((profile) => (profile.tags ?? []).length === 0);
        const filterKeys = hasUntagged ? [...allTags, this.untaggedFilterTag] : allTags;
        const allSelected = filterKeys.every((tag) => this.selectedSessionTags.has(tag));
        if (filterKeys.length === 0 || allSelected) {
            return profiles;
        }
        return profiles.filter((profile) => {
            const tags = profile.tags ?? [];
            if (tags.length === 0) {
                return this.selectedSessionTags.has(this.untaggedFilterTag);
            }
            return tags.some((tag) => this.selectedSessionTags.has(tag));
        });
    }

    private commitSessionTagDraft(): void {
        const tag = this.sessionTagDraft.trim();
        if (tag && !this.sessionForm.tags.includes(tag)) {
            this.sessionForm.tags = [...this.sessionForm.tags, tag];
        }
        this.sessionTagDraft = '';
        this.sessionTagInputVisible = false;
    }
}

function inferDirectory(entries: FileEntry[], requestedPath: string): string {
    if (requestedPath) {
        return requestedPath;
    }
    const firstEntry = entries[0];
    if (!firstEntry) {
        return '.';
    }
    return parentPath(firstEntry.path);
}

function parentPath(value: string): string {
    if (!value || value === '.' || value === '/') {
        return '.';
    }
    const normalized = value.endsWith('/') ? value.slice(0, -1) : value;
    const index = normalized.lastIndexOf('/');
    if (index <= 0) {
        return '.';
    }
    return normalized.slice(0, index);
}

function parentVaultPath(value: string): string {
    if (!value || value === '/') {
        return '';
    }
    const normalized = value.endsWith('/') ? value.slice(0, -1) : value;
    const index = normalized.lastIndexOf('/');
    if (index < 0) {
        return '';
    }
    return normalized.slice(0, index);
}

function formatBytes(value: number): string {
    if (!Number.isFinite(value) || value <= 0) {
        return '0 B';
    }
    const units = ['B', 'KB', 'MB', 'GB'];
    let size = value;
    let unitIndex = 0;
    while (size >= 1024 && unitIndex < units.length - 1) {
        size /= 1024;
        unitIndex++;
    }
    return `${size.toFixed(size >= 10 || unitIndex === 0 ? 0 : 1)} ${units[unitIndex]}`;
}

function escapeHtml(value: string): string {
    return value
        .replaceAll('&', '&amp;')
        .replaceAll('<', '&lt;')
        .replaceAll('>', '&gt;')
        .replaceAll('"', '&quot;')
        .replaceAll("'", '&#39;');
}

function escapeClassName(value: string): string {
    return value.replace(/[^a-zA-Z0-9_-]/g, '-');
}

function formatError(prefix: string, error: unknown): string {
    if (error instanceof Error) {
        return `${prefix}: ${error.message}`;
    }
    if (typeof error === 'string') {
        return `${prefix}: ${error}`;
    }
    return prefix;
}

function renderMarkdown(value: string): string {
    const placeholders: string[] = [];
    let escaped = escapeHtml(value).replaceAll('\r\n', '\n');
    escaped = escaped.replace(/```(?:[^\n`]*)\n([\s\S]*?)```/g, (_match, code: string) => {
        const index = placeholders.push(`<pre class="md-block-code"><code>${code.replace(/\n+$/g, '')}</code></pre>`) - 1;
        return `@@MD_CODE_BLOCK_${index}@@`;
    });

    const lines = escaped.split('\n');
    const blocks: string[] = [];
    let inList = false;

    const closeList = () => {
        if (inList) {
            blocks.push('</ul>');
            inList = false;
        }
    };

    for (const rawLine of lines) {
        const line = rawLine.trimEnd();
        const trimmed = line.trim();
        if (!trimmed) {
            closeList();
            continue;
        }
        const codePlaceholder = trimmed.match(/^@@MD_CODE_BLOCK_(\d+)@@$/);
        if (codePlaceholder) {
            closeList();
            blocks.push(trimmed);
            continue;
        }
        const heading = trimmed.match(/^(#{1,6})\s+(.+)$/);
        if (heading) {
            closeList();
            const level = heading[1].length;
            blocks.push(`<h${level}>${applyInlineMarkdown(heading[2])}</h${level}>`);
            continue;
        }
        const quote = trimmed.match(/^>\s?(.*)$/);
        if (quote) {
            closeList();
            blocks.push(`<blockquote>${applyInlineMarkdown(quote[1])}</blockquote>`);
            continue;
        }
        const listItem = trimmed.match(/^[-*]\s+(.+)$/);
        if (listItem) {
            if (!inList) {
                blocks.push('<ul>');
                inList = true;
            }
            blocks.push(`<li>${applyInlineMarkdown(listItem[1])}</li>`);
            continue;
        }
        closeList();
        blocks.push(`<p>${applyInlineMarkdown(trimmed)}</p>`);
    }
    closeList();

    let html = blocks.join('');
    html = html.replace(/@@MD_CODE_BLOCK_(\d+)@@/g, (_match, idx: string) => placeholders[Number(idx)] ?? '');
    return html;
}

function applyInlineMarkdown(value: string): string {
    return value
        .replace(/\[([^\]\n]+)\]\((https?:\/\/[^\s)]+)\)/g, '<a href="$2" target="_blank" rel="noopener noreferrer">$1</a>')
        .replace(/\*\*([^*\n]+)\*\*/g, '<strong>$1</strong>')
        .replace(/\*([^*\n]+)\*/g, '<em>$1</em>')
        .replace(/`([^`\n]+)`/g, '<code class="md-inline-code">$1</code>');
}

void new OpsyShell().bootstrap();
