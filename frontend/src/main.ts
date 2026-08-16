import './style.css';
import './app.css';
import '@xterm/xterm/css/xterm.css';

import { Terminal } from '@xterm/xterm';
import { FitAddon } from '@xterm/addon-fit';
import { WebLinksAddon } from '@xterm/addon-web-links';
import {
    CancelModelDownload,
    CloseSession,
    ConnectSSH,
    CreateSessionProfile,
    DeleteSessionProfile,
    DisconnectSSH,
    DownloadLocalModelWithProgress,
    GetShellState,
    LaunchSession,
    ListSFTPFiles,
    NavigateSFTP,
    ImportSSHConfig,
    ReadSFTPFile,
    SaveCloudProvider,
    SaveSFTPFile,
    SelectAIProvider,
    SendSSHInput,
    StartLocalModel,
    ResizeTerminal,
    UpdateSettings,
} from '../wailsjs/go/main/App';
import { EventsOn } from '../wailsjs/runtime/runtime';
import type { ai as aiModels, app as appModels, sessions, settings as settingsModels, sftp as sftpModels } from '../wailsjs/go/models';

type ShellState = appModels.ShellState;
type RuntimeSession = appModels.RuntimeSessionView;
type SessionProfile = sessions.ProfileInput;
type AIProvider = aiModels.ProviderDescriptor;
type FileEntry = sftpModels.FileEntry;

type SessionFormState = {
    name: string;
    group: string;
    host: string;
    port: string;
    username: string;
    password: string;
    protocolId: string;
    tags: string;
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
    editorPath: string;
    editorContent: string;
    editorLoading: boolean;
    editorSaving: boolean;
    editorDirty: boolean;
    editorError: string;
};

type ModelProgressState = {
    downloaded: number;
    total: number;
    percent: number;
    active: boolean;
    error: string;
};

type TerminalState = {
    terminal: Terminal;
    fitAddon: FitAddon;
    wrapper: HTMLDivElement;
    inner: HTMLDivElement;
    opened: boolean;
    unsubscribe: (() => void) | null;
};

type Theme = 'dark' | 'light';
type SettingsTab = 'ai' | 'sshconfig' | 'theme' | 'logs';
type SessionModalTab = 'basic' | 'advanced';

type LogEntry = {
    level: string;
    message: string;
    time: string;
};

const THEME_KEY = 'opsy-theme';

const root = document.querySelector<HTMLDivElement>('#app');

// Encode to Base64 (Unicode Safe)
function encodeBase64(str: string): string {
  const bytes = new TextEncoder().encode(str);
  const binString = String.fromCodePoint(...bytes);
  return btoa(binString);
}

// Decode from Base64 (Unicode Safe)
function decodeBase64(base64: string): string {
  const binString = atob(base64);
  const bytes = Uint8Array.from(binString, (m) => m.codePointAt(0)!);
  return new TextDecoder().decode(bytes);
}

class OpsyShell {
    private shellState: ShellState | null = null;
    private activeTabId = '';
    private errorMessage = '';
    private showSessionModal = false;
    private sessionModalTab: SessionModalTab = 'basic';
    private showSettingsModal = false;
    private settingsTab: SettingsTab = 'ai';
    private sessionForm: SessionFormState = this.defaultSessionForm();
    private terminals = new Map<string, TerminalState>();
    private sftpState: SFTPState = {
        tabId: null,
        path: '',
        entries: [],
        loading: false,
        error: '',
        editorPath: '',
        editorContent: '',
        editorLoading: false,
        editorSaving: false,
        editorDirty: false,
        editorError: '',
    };
    private sshConfigDraft = '';
    private modelProgress: ModelProgressState = { downloaded: 0, total: 0, percent: 0, active: false, error: '' };
    private theme: Theme;
    private logEntries: LogEntry[] = [];
    private readonly MAX_LOG_ENTRIES = 200;

    constructor() {
        const saved = localStorage.getItem(THEME_KEY) as Theme | null;
        this.theme = saved === 'light' ? 'light' : 'dark';
        this.applyTheme();
    }

    private applyTheme(): void {
        if (this.theme === 'light') {
            document.documentElement.classList.add('light');
        } else {
            document.documentElement.classList.remove('light');
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
        EventsOn('model:progress', (...payload: unknown[]) => {
            const data = payload[0] as { downloaded?: number; total?: number; percent?: number } | undefined;
            this.modelProgress = {
                downloaded: data?.downloaded ?? 0,
                total: data?.total ?? 0,
                percent: data?.percent ?? 0,
                active: true,
                error: '',
            };
            this.render();
        });
        EventsOn('model:error', (...payload: unknown[]) => {
            const data = payload[0] as { error?: string } | undefined;
            this.modelProgress.error = data?.error ?? 'Model download failed';
            this.modelProgress.active = false;
            this.render();
        });
        EventsOn('app:log', (...payload: unknown[]) => {
            const data = payload[0] as { level?: string; message?: string; time?: string } | undefined;
            if (!data?.message) return;
            this.logEntries.push({
                level: data.level ?? 'info',
                message: data.message,
                time: data.time ?? new Date().toISOString(),
            });
            if (this.logEntries.length > this.MAX_LOG_ENTRIES) {
                this.logEntries = this.logEntries.slice(-this.MAX_LOG_ENTRIES);
            }
            if (this.shellState?.settings.showLogPanel) {
                this.render();
            }
        });
    }

    private async refresh(errorMessage = this.errorMessage): Promise<void> {
        this.errorMessage = errorMessage;
        this.shellState = await GetShellState();
        const preferredTabID = this.activeTabId || this.shellState.workspace.layout.activeTabId || '';
        this.activeTabId = this.pickActiveTabID(preferredTabID);
        if (this.sftpState.tabId !== this.activeTabId) {
            this.sftpState = this.defaultSFTPState(this.activeTabId || null);
        }
        this.render();
    }

    private render(): void {
        if (!root || !this.shellState) {
            return;
        }

        root.innerHTML = `
            <div class="shell">
                <aside class="panel sidebar-panel">
                    <div class="panel-header">
                        <div>
                            <div class="eyebrow">Session manager</div>
                            <h1>opsy</h1>
                        </div>
                        <div style="display:flex;gap:0.5rem;align-items:center;">
                            <button class="icon-button" data-open-session-modal title="New session">+</button>
                            <button class="icon-button" data-open-settings-modal title="Settings">⚙</button>
                        </div>
                    </div>
                    <section class="section">
                        <div class="section-heading">
                            <span class="section-title">Sessions</span>
                        </div>
                        <div class="session-list">
                            ${this.renderSessionProfiles()}
                        </div>
                    </section>
                    <section class="section sftp-section">
                        <div class="section-heading">
                            <span class="section-title">SFTP Browser</span>
                            <div class="section-actions">
                                ${this.renderSFTPActions()}
                            </div>
                        </div>
                        ${this.renderSFTPBrowser()}
                    </section>
                </aside>

                <main class="panel workspace-panel">
                    <div class="tab-bar">${this.renderTabs()}</div>
                    ${this.errorMessage ? `<div class="error-banner">${escapeHtml(this.errorMessage)}</div>` : ''}
                    <div class="terminal-shell">
                        <div id="terminal-host" class="terminal-container"></div>
                    </div>
                </main>

                <aside class="panel assistant-panel">
                    <div class="panel-header">
                        <div>
                            <div class="eyebrow">AI assistant</div>
                            <h2>Chat</h2>
                        </div>
                    </div>
                    ${this.modelProgress.active || this.modelProgress.error ? this.renderProgress() : ''}
                    <section class="section chat-section">
                        <div class="section-title">Assistant</div>
                        ${this.renderMessages()}
                    </section>
                </aside>
            </div>
            ${this.shellState.settings.showLogPanel ? this.renderLogPanel() : ''}
            ${this.showSessionModal ? this.renderSessionModal() : ''}
            ${this.showSettingsModal ? this.renderSettingsModal() : ''}
        `;

        this.bindEvents();
        this.attachActiveTerminal();
        this.scrollLogPanelToBottom();
    }

    private bindEvents(): void {
        root?.querySelector<HTMLButtonElement>('[data-open-session-modal]')?.addEventListener('click', () => {
            this.showSessionModal = true;
            this.sessionModalTab = 'basic';
            this.render();
        });

        root?.querySelector<HTMLButtonElement>('[data-open-settings-modal]')?.addEventListener('click', () => {
            this.showSettingsModal = true;
            this.render();
        });

        root?.querySelectorAll<HTMLButtonElement>('[data-settings-tab]').forEach((button) => {
            button.addEventListener('click', () => {
                this.settingsTab = (button.dataset.settingsTab as SettingsTab) ?? 'ai';
                this.render();
            });
        });

        root?.querySelectorAll<HTMLButtonElement>('[data-session-modal-tab]').forEach((button) => {
            button.addEventListener('click', () => {
                this.sessionModalTab = (button.dataset.sessionModalTab as SessionModalTab) ?? 'basic';
                this.render();
            });
        });

        root?.querySelectorAll<HTMLButtonElement>('[data-open-profile]').forEach((button) => {
            button.addEventListener('click', async () => {
                const profileID = button.dataset.openProfile;
                if (!profileID || !this.shellState) {
                    return;
                }
                await this.openProfile(profileID);
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
                this.errorMessage = formatError('Unable to import SSH config', error);
                this.render();
            }
        });

        root?.querySelectorAll<HTMLButtonElement>('[data-delete-profile]').forEach((button) => {
            button.addEventListener('click', async () => {
                const profileID = button.dataset.deleteProfile;
                if (!profileID) {
                    return;
                }
                await this.runAction(async () => DeleteSessionProfile(profileID), 'Unable to delete session profile');
            });
        });

        root?.querySelectorAll<HTMLButtonElement>('[data-tab-id]').forEach((button) => {
            button.addEventListener('click', () => {
                const tabID = button.dataset.tabId;
                if (!tabID) {
                    return;
                }
                this.activeTabId = tabID;
                this.render();
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

        root?.querySelector<HTMLButtonElement>('[data-open-sftp]')?.addEventListener('click', async () => {
            const activeTab = this.activeTab();
            if (!activeTab) {
                return;
            }
            await this.loadSFTP(activeTab.id, '');
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

        root?.querySelectorAll<HTMLButtonElement>('[data-sftp-dir]').forEach((button) => {
            button.addEventListener('click', async () => {
                const targetPath = button.dataset.sftpDir;
                const activeTab = this.activeTab();
                if (!targetPath || !activeTab) {
                    return;
                }
                await this.loadSFTP(activeTab.id, targetPath);
            });

            root?.querySelectorAll<HTMLButtonElement>('[data-sftp-file]').forEach((button) => {
                button.addEventListener('click', async () => {
                    const targetPath = button.dataset.sftpFile;
                    const activeTab = this.activeTab();
                    if (!targetPath || !activeTab) {
                        return;
                    }
                    await this.openRemoteFile(activeTab.id, targetPath);
                });
            });

            root?.querySelector<HTMLButtonElement>('[data-save-remote-file]')?.addEventListener('click', async () => {
                const activeTab = this.activeTab();
                if (!activeTab || !this.sftpState.editorPath) {
                    return;
                }
                await this.saveRemoteFile(activeTab.id);
            });

            root?.querySelector<HTMLTextAreaElement>('[data-remote-editor]')?.addEventListener('input', (event) => {
                this.sftpState.editorContent = (event.currentTarget as HTMLTextAreaElement).value;
                this.sftpState.editorDirty = true;
            });
        });

        root?.querySelectorAll<HTMLButtonElement>('[data-select-provider]').forEach((button) => {
            button.addEventListener('click', async () => {
                const providerID = button.dataset.selectProvider;
                if (!providerID) {
                    return;
                }
                await this.runAction(async () => SelectAIProvider(providerID), 'Unable to switch AI provider');
            });
        });

        root?.querySelector<HTMLButtonElement>('[data-download-model]')?.addEventListener('click', async () => {
            this.modelProgress = { downloaded: 0, total: 0, percent: 0, active: true, error: '' };
            this.render();
            try {
                await DownloadLocalModelWithProgress();
                this.modelProgress.active = false;
                await this.refresh('');
            } catch (error) {
                this.modelProgress.active = false;
                this.modelProgress.error = formatError('Unable to download local model', error);
                this.errorMessage = this.modelProgress.error;
                this.render();
            }
        });

        root?.querySelectorAll<HTMLButtonElement>('[data-cancel-download]').forEach((button) => {
            button.addEventListener('click', async () => {
                try {
                    await CancelModelDownload();
                } catch {
                    // ignore
                }
                this.modelProgress = { downloaded: 0, total: 0, percent: 0, active: false, error: '' };
                this.render();
            });
        });

        root?.querySelector<HTMLButtonElement>('[data-start-model]')?.addEventListener('click', async () => {
            await this.runAction(async () => StartLocalModel(), 'Unable to start local model');
        });

        root?.querySelector<HTMLFormElement>('[data-cloud-form]')?.addEventListener('submit', async (event) => {
            event.preventDefault();
            const form = event.currentTarget as HTMLFormElement;
            const endpoint = form.querySelector<HTMLInputElement>('input[name="endpoint"]')?.value ?? '';
            const token = form.querySelector<HTMLInputElement>('input[name="token"]')?.value ?? '';
            await this.runAction(async () => SaveCloudProvider(endpoint, token), 'Unable to save cloud provider');
        });

        root?.querySelectorAll<HTMLElement>('[data-close-modal]').forEach((button) => {
            button.addEventListener('click', () => {
                this.showSessionModal = false;
                this.showSettingsModal = false;
                this.render();
            });
        });

        root?.querySelectorAll<HTMLButtonElement>('[data-set-theme]').forEach((button) => {
            button.addEventListener('click', () => {
                const t = button.dataset.setTheme as Theme;
                if (t === 'light' || t === 'dark') {
                    this.theme = t;
                    this.applyTheme();
                    this.render();
                }
            });
        });

        root?.querySelectorAll<HTMLButtonElement>('[data-set-log-panel]').forEach((button) => {
            button.addEventListener('click', async () => {
                const show = button.dataset.setLogPanel === 'true';
                if (!this.shellState) return;
                const updated = { ...this.shellState.settings, showLogPanel: show } as unknown as settingsModels.AppSettings;
                await this.runAction(async () => UpdateSettings(updated), 'Unable to save log panel setting');
            });
        });

        root?.querySelectorAll<HTMLButtonElement>('[data-set-log-level]').forEach((button) => {
            button.addEventListener('click', async () => {
                const level = button.dataset.setLogLevel ?? 'info';
                if (!this.shellState) return;
                const updated = { ...this.shellState.settings, logLevel: level } as unknown as settingsModels.AppSettings;
                await this.runAction(async () => UpdateSettings(updated), 'Unable to save log level setting');
            });
        });

        root?.querySelector<HTMLButtonElement>('[data-hide-log-panel]')?.addEventListener('click', async () => {
            if (!this.shellState) return;
            const updated = { ...this.shellState.settings, showLogPanel: false } as unknown as settingsModels.AppSettings;
            await this.runAction(async () => UpdateSettings(updated), 'Unable to hide log panel');
        });

        root?.querySelector<HTMLFormElement>('[data-session-form]')?.addEventListener('submit', async (event) => {
            event.preventDefault();
            const form = event.currentTarget as HTMLFormElement;
            const formData = new FormData(form);
            const password: string = encodeBase64(String(formData.get('password') ?? ''));
            const profile: SessionProfile = {
                id: '',
                name: String(formData.get('name') ?? ''),
                group: String(formData.get('group') ?? ''),
                host: String(formData.get('host') ?? ''),
                port: Number(formData.get('port') ?? 22),
                username: String(formData.get('username') ?? ''),
                password: password,
                protocolId: String(formData.get('protocolId') ?? 'ssh'),
                tags: String(formData.get('tags') ?? '').split(',').map((tag) => tag.trim()).filter(Boolean),
                favorite: false,
            };
            const proxyJump = String(formData.get('proxyJump') ?? '').trim();
            const localForwards = String(formData.get('localForwards') ?? '').trim();
            const useSSHAgent = formData.get('useSSHAgent') === 'on';
            const options: Record<string, string> = {};
            if (proxyJump) {
                options.proxy_jump = proxyJump;
            }
            if (localForwards) {
                options.local_forwards = localForwards;
            }
            if (useSSHAgent) {
                options.use_ssh_agent = 'true';
            }
            if (Object.keys(options).length > 0) {
                profile.options = options;
            }
            try {
                await CreateSessionProfile(profile);
                this.showSessionModal = false;
                this.sessionForm = this.defaultSessionForm();
                await this.refresh('');
            } catch (error) {
                this.errorMessage = formatError('Unable to save session profile', error);
                this.render();
            }
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
            await this.refresh('');
            if (profile.protocolId === 'ssh') {
                this.ensureTerminalSubscription(tab.id);
                await ConnectSSH(tab.id, profileID);
                this.fitActiveTerminal();
            } else if (profile.protocolId === 'rdp') {
                this.errorMessage = 'RDP tab created. Native desktop stream is not yet wired in this build.';
                this.render();
            }
        } catch (error) {
            this.errorMessage = formatError('Unable to open session', error);
            this.render();
        }
    }

    private async closeTab(tabID: string): Promise<void> {
        const terminal = this.terminals.get(tabID);
        terminal?.unsubscribe?.();
        terminal?.terminal.dispose();
        terminal?.wrapper.remove();
        this.terminals.delete(tabID);
        try {
            await DisconnectSSH(tabID);
        } catch {
            // ignored - backend CloseSession will clean up too.
        }
        await this.runAction(async () => CloseSession(tabID), 'Unable to close session');
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
            };
        } catch (error) {
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
            editorPath: targetPath,
            editorLoading: true,
            editorError: '',
        };
        this.render();
        try {
            const content = await ReadSFTPFile(tabID, targetPath);
            this.sftpState = {
                ...this.sftpState,
                editorPath: targetPath,
                editorContent: content,
                editorLoading: false,
                editorDirty: false,
                editorError: '',
            };
        } catch (error) {
            this.sftpState = {
                ...this.sftpState,
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
            this.sftpState = {
                ...this.sftpState,
                editorSaving: false,
                editorError: formatError('Unable to save remote file', error),
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
            terminalState.terminal.onData((data) => {
                void SendSSHInput(activeTab.id, data).catch((error) => {
                    this.errorMessage = formatError('Unable to send terminal input', error);
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
            terminal.write(data?.data ?? '');
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

    private scrollLogPanelToBottom(): void {
        const body = document.querySelector<HTMLDivElement>('#log-panel-body');
        if (body) {
            body.scrollTop = body.scrollHeight;
        }
    }

    private renderLogPanel(): string {
        const logLevelOrder: Record<string, number> = { debug: 0, info: 1, warn: 2, error: 3 };
        const currentLevel = this.shellState?.settings.logLevel ?? 'info';
        const currentOrder = logLevelOrder[currentLevel] ?? 1;
        const filtered = this.logEntries.filter((e) => (logLevelOrder[e.level] ?? 1) >= currentOrder);
        const entries = filtered.slice(-50);
        const rows = entries.length > 0
            ? entries.map((e) => {
                const time = e.time ? new Date(e.time).toLocaleTimeString() : '';
                return `<div class="log-entry log-level-${escapeHtml(e.level)}"><span class="log-time">${escapeHtml(time)}</span><span class="log-level">${escapeHtml(e.level.toUpperCase())}</span><span class="log-message">${escapeHtml(e.message)}</span></div>`;
            }).join('')
            : '<div class="log-empty">No log entries</div>';
        return `
            <div class="log-panel" id="log-panel">
                <div class="log-panel-header">
                    <span class="log-panel-title">Logs</span>
                    <button class="icon-button" data-hide-log-panel title="Hide log panel">×</button>
                </div>
                <div class="log-panel-body" id="log-panel-body">${rows}</div>
            </div>
        `;
    }

    private renderSessionProfiles(): string {
        if (!this.shellState || this.shellState.sessionProfiles.length === 0) {
            return '<div class="empty-state">No saved sessions yet.</div>';
        }
        return this.shellState.sessionProfiles.map((profile) => `
            <article class="session-card">
                <div>
                    <div class="session-title-row">
                        <strong>${escapeHtml(profile.name)}</strong>
                        <span class="pill small">${escapeHtml(profile.protocolId.toUpperCase())}</span>
                    </div>
                    <div class="session-meta">${escapeHtml(profile.group || 'Ungrouped')} · ${escapeHtml(profile.username)}@${escapeHtml(profile.host)}:${escapeHtml(String(profile.port))}</div>
                    <div class="session-tags">${profile.tags.map((tag) => `<span>${escapeHtml(tag)}</span>`).join('')}</div>
                </div>
                <div class="session-actions">
                    <button class="action-button" data-open-profile="${escapeHtml(profile.id)}">Open</button>
                    <button class="icon-button danger" data-delete-profile="${escapeHtml(profile.id)}">×</button>
                </div>
            </article>
        `).join('');
    }

    private renderTabs(): string {
        if (!this.shellState || this.shellState.activeSessions.length === 0) {
            return '<div class="tab empty">No active sessions</div>';
        }
        return this.shellState.activeSessions.map((tab) => `
            <button class="tab ${tab.id === this.activeTabId ? 'active' : ''}" data-tab-id="${escapeHtml(tab.id)}">
                <span>${escapeHtml(tab.title)}</span>
                <small>${escapeHtml(tab.status)}</small>
                <span class="tab-close" data-close-tab="${escapeHtml(tab.id)}">×</span>
            </button>
        `).join('');
    }

    private renderSFTPActions(): string {
        const activeTab = this.activeTab();
        if (!activeTab || activeTab.protocolId !== 'ssh') {
            return '<span class="section-copy">Select an SSH tab</span>';
        }
        const openLabel = this.sftpState.entries.length === 0 ? 'Open SFTP' : 'Refresh';
        return `
            <button class="action-button secondary" data-open-sftp>${openLabel}</button>
            ${this.sftpState.entries.length > 0 ? '<button class="action-button secondary" data-refresh-sftp>Reload</button>' : ''}
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
            return '<div class="empty-state">Click "Open SFTP" to browse the active session.</div>';
        }
        return `
            <div class="sftp-path-row">
                <span class="sftp-path">${escapeHtml(this.sftpState.path || '.')}</span>
                <button class="action-button secondary" data-sftp-up>..</button>
            </div>
            <div class="sftp-list">
                ${this.sftpState.entries.map((entry) => entry.isDir
                    ? `<button class="sftp-entry sftp-dir" data-sftp-dir="${escapeHtml(entry.path)}"><span>${escapeHtml(entry.name)}</span><small>${escapeHtml(entry.modTime)}</small></button>`
                    : `<button class="sftp-entry sftp-file" data-sftp-file="${escapeHtml(entry.path)}"><span>${escapeHtml(entry.name)}</span><small>${escapeHtml(formatFileMeta(entry))}</small></button>`).join('')}
            </div>
            ${this.renderRemoteEditor()}
        `;
    }

    private renderRemoteEditor(): string {
        if (!this.sftpState.editorPath) {
            return '<div class="empty-state">Select a file to open remote editor.</div>';
        }
        if (this.sftpState.editorLoading) {
            return '<div class="empty-state">Loading file…</div>';
        }
        return `
            <div class="remote-editor">
                <div class="sftp-path-row">
                    <span class="sftp-path">${escapeHtml(this.sftpState.editorPath)}</span>
                    <button class="action-button secondary" data-save-remote-file ${this.sftpState.editorSaving ? 'disabled' : ''}>${this.sftpState.editorSaving ? 'Saving…' : 'Save'}</button>
                </div>
                ${this.sftpState.editorError ? `<div class="error-banner compact">${escapeHtml(this.sftpState.editorError)}</div>` : ''}
                <textarea class="remote-editor-input" data-remote-editor>${escapeHtml(this.sftpState.editorContent)}</textarea>
            </div>
        `;
    }

    private renderAIProviders(): string {
        if (!this.shellState) {
            return '';
        }
        return this.shellState.ai.providers.map((provider) => `
            <article class="provider-card ${provider.selected ? 'selected-provider' : ''}">
                <div>
                    <strong>${escapeHtml(provider.name)}</strong>
                    <div class="session-meta">${escapeHtml(provider.model)} · ${escapeHtml(provider.status)}</div>
                </div>
                <button class="action-button secondary" data-select-provider="${escapeHtml(provider.id)}">${provider.selected ? 'Selected' : 'Use'}</button>
            </article>
        `).join('');
    }

    private renderProviderSetup(): string {
        const provider = this.selectedProvider();
        if (!provider) {
            return '<div class="empty-state">Select an AI provider to configure it.</div>';
        }
        if (provider.class === 'local') {
            return `
                <div class="provider-setup-card">
                    <div class="section-title">Local model</div>
                    <div class="session-meta">${escapeHtml(provider.localPath || 'Model not downloaded yet')}</div>
                    <div class="provider-form-actions">
                        <button class="action-button" data-download-model ${this.modelProgress.active ? 'disabled' : ''}>
                            ${this.modelProgress.active ? 'Downloading…' : 'Download Qwen3'}
                        </button>
                        ${this.modelProgress.active ? `<button class="action-button secondary" data-cancel-download>Cancel</button>` : ''}
                        <button class="action-button secondary" data-start-model ${provider.localPath ? '' : 'disabled'}>Start local model</button>
                    </div>
                    ${this.modelProgress.active ? `
                        <div class="progress-bar"><div class="progress-fill" style="width: ${Math.max(0, Math.min(100, this.modelProgress.percent))}%"></div></div>
                        <div class="session-meta">${escapeHtml(`${formatBytes(this.modelProgress.downloaded)} / ${formatBytes(this.modelProgress.total)}`)}</div>
                    ` : ''}
                    ${this.modelProgress.error ? `<div class="error-banner compact">${escapeHtml(this.modelProgress.error)}</div>` : ''}
                </div>
            `;
        }
        return `
            <form class="provider-form" data-cloud-form>
                <label>
                    <span>Endpoint</span>
                    <input type="url" name="endpoint" value="${escapeHtml(provider.endpoint || '')}" placeholder="https://api.example.com/v1" required />
                </label>
                <label>
                    <span>API token</span>
                    <input type="password" name="token" placeholder="sk-..." required />
                </label>
                <div class="provider-form-actions">
                    <button class="action-button" type="submit">Save cloud provider</button>
                </div>
            </form>
        `;
    }

    private renderMessages(): string {
        if (!this.shellState) {
            return '';
        }
        if (!this.hasConfiguredProvider()) {
            return '<div class="empty-state">Configure a provider in Settings to start chatting.</div>';
        }
        return this.shellState.ai.messages.map((message) => `
            <div class="message ${escapeClassName(message.role)}">${escapeHtml(message.content)}</div>
        `).join('');
    }

    private renderProgress(): string {
        return `
            <section class="section">
                <div class="section-title">Model download</div>
                <div class="progress-bar"><div class="progress-fill" style="width: ${Math.max(0, Math.min(100, this.modelProgress.percent))}%"></div></div>
                <div class="session-meta">${escapeHtml(this.modelProgress.error || `${formatBytes(this.modelProgress.downloaded)} / ${formatBytes(this.modelProgress.total)}`)}</div>
                ${this.modelProgress.active ? `
                    <div style="margin-top:0.5rem;">
                        <button class="action-button secondary" data-cancel-download>Stop download</button>
                    </div>
                ` : ''}
            </section>
        `;
    }

    private renderSettingsModal(): string {
        const tabs: Array<{ id: SettingsTab; label: string }> = [
            { id: 'ai', label: 'AI' },
            { id: 'sshconfig', label: 'SSH Config' },
            { id: 'theme', label: 'Theme' },
            { id: 'logs', label: 'Logs' },
        ];
        const currentLogLevel = this.shellState?.settings.logLevel ?? 'info';
        const showLogPanel = this.shellState?.settings.showLogPanel ?? false;
        const logLevels = [
            { value: 'debug', label: 'Debug' },
            { value: 'info', label: 'Info' },
            { value: 'warn', label: 'Warning' },
            { value: 'error', label: 'Error' },
        ];
        return `
            <div class="modal-overlay">
                <div class="modal-dialog wide">
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
                            <div class="section-title">AI Providers</div>
                            <div class="provider-list">
                                ${this.renderAIProviders()}
                            </div>
                            ${this.renderProviderSetup()}
                        </div>
                        <div class="modal-tab-panel ${this.settingsTab === 'sshconfig' ? 'active' : ''}">
                            <div class="section-title">Import SSH Config</div>
                            <div class="import-box">
                                <label class="import-label" for="ssh-config-import">Paste SSH config block</label>
                                <textarea id="ssh-config-import" data-ssh-config-import placeholder="Host prod&#10;  HostName prod.internal&#10;  User ops&#10;  ProxyJump bastion">${escapeHtml(this.sshConfigDraft)}</textarea>
                                <button class="action-button secondary" data-import-ssh-config>Import</button>
                            </div>
                        </div>
                        <div class="modal-tab-panel ${this.settingsTab === 'theme' ? 'active' : ''}">
                            <div class="section-title">Appearance</div>
                            <div class="theme-toggle-row">
                                <span>Theme</span>
                                <div class="theme-switch">
                                    <button class="${this.theme === 'dark' ? 'active' : ''}" data-set-theme="dark">🌙 Dark</button>
                                    <button class="${this.theme === 'light' ? 'active' : ''}" data-set-theme="light">☀ Light</button>
                                </div>
                            </div>
                        </div>
                        <div class="modal-tab-panel ${this.settingsTab === 'logs' ? 'active' : ''}">
                            <div class="section-title">Log Panel</div>
                            <div class="theme-toggle-row">
                                <span>Show log panel</span>
                                <div class="theme-switch">
                                    <button class="${showLogPanel ? 'active' : ''}" data-set-log-panel="true">On</button>
                                    <button class="${!showLogPanel ? 'active' : ''}" data-set-log-panel="false">Off</button>
                                </div>
                            </div>
                            <div class="theme-toggle-row" style="margin-top:1rem;">
                                <span>Log level</span>
                                <div class="theme-switch">
                                    ${logLevels.map((l) => `<button class="${currentLogLevel === l.value ? 'active' : ''}" data-set-log-level="${l.value}">${l.label}</button>`).join('')}
                                </div>
                            </div>
                        </div>
                    </div>
                </div>
            </div>
        `;
    }

    private renderSessionModal(): string {
        const tabs: Array<{ id: SessionModalTab; label: string }> = [
            { id: 'basic', label: 'Basic' },
            { id: 'advanced', label: 'Advanced' },
        ];
        return `
            <div class="modal-overlay">
                <div class="modal-dialog">
                    <div class="panel-header compact-header">
                        <div>
                            <div class="eyebrow">New session</div>
                            <h2>Create session profile</h2>
                        </div>
                        <button class="icon-button" data-close-modal>×</button>
                    </div>
                    <nav class="modal-tabs">
                        ${tabs.map((t) => `<button class="modal-tab ${this.sessionModalTab === t.id ? 'active' : ''}" data-session-modal-tab="${t.id}">${t.label}</button>`).join('')}
                    </nav>
                    <div class="modal-body">
                        <form class="session-form" data-session-form>
                            <div class="modal-tab-panel ${this.sessionModalTab === 'basic' ? 'active' : ''}">
                                <label><span>Name</span><input name="name" value="${escapeHtml(this.sessionForm.name)}" required /></label>
                                <label><span>Group</span><input name="group" value="${escapeHtml(this.sessionForm.group)}" /></label>
                                <label><span>Host</span><input name="host" value="${escapeHtml(this.sessionForm.host)}" required /></label>
                                <label><span>Port</span><input name="port" type="number" value="${escapeHtml(this.sessionForm.port)}" min="1" required /></label>
                                <label><span>Username</span><input name="username" value="${escapeHtml(this.sessionForm.username)}" required /></label>
                                <label><span>Password</span><input name="password" type="password" value="${escapeHtml(decodeBase64(this.sessionForm.password))}" /></label>
                                <label>
                                    <span>Protocol</span>
                                    <select name="protocolId">
                                        <option value="ssh" ${this.sessionForm.protocolId === 'ssh' ? 'selected' : ''}>SSH</option>
                                        <option value="sftp" ${this.sessionForm.protocolId === 'sftp' ? 'selected' : ''}>SFTP</option>
                                        <option value="rdp" ${this.sessionForm.protocolId === 'rdp' ? 'selected' : ''}>RDP</option>
                                    </select>
                                </label>
                                <label><span>Tags</span><input name="tags" value="${escapeHtml(this.sessionForm.tags)}" placeholder="prod, linux" /></label>
                            </div>
                            <div class="modal-tab-panel ${this.sessionModalTab === 'advanced' ? 'active' : ''}">
                                <label><span>ProxyJump</span><input name="proxyJump" value="${escapeHtml(this.sessionForm.proxyJump)}" placeholder="bastion or user@bastion:22" /></label>
                                <label><span>Local tunnels</span><input name="localForwards" value="${escapeHtml(this.sessionForm.localForwards)}" placeholder="15432:db.internal:5432,18080:127.0.0.1:8080" /></label>
                                <label class="inline-check"><span>Use SSH agent</span><input name="useSSHAgent" type="checkbox" ${this.sessionForm.useSSHAgent ? 'checked' : ''} /></label>
                            </div>
                            <div style="padding: 0 1.5rem 1.25rem; display:flex; gap:0.75rem; justify-content:flex-end;">
                                <button type="button" class="action-button secondary" data-close-modal>Cancel</button>
                                <button type="submit" class="action-button">Save</button>
                            </div>
                        </form>
                    </div>
                </div>
            </div>
        `;
    }

    private pickActiveTabID(preferredID: string): string {
        const tabs = this.shellState?.activeSessions ?? [];
        if (tabs.some((tab) => tab.id === preferredID)) {
            return preferredID;
        }
        return tabs[0]?.id ?? '';
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

    private async runAction(action: () => Promise<void>, prefix: string): Promise<void> {
        try {
            await action();
            await this.refresh('');
        } catch (error) {
            this.errorMessage = formatError(prefix, error);
            this.render();
        }
    }

    private defaultSessionForm(): SessionFormState {
        return {
            name: '',
            group: '',
            host: '',
            port: '22',
            username: '',
            password: '',
            protocolId: 'ssh',
            tags: '',
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
            editorPath: '',
            editorContent: '',
            editorLoading: false,
            editorSaving: false,
            editorDirty: false,
            editorError: '',
        };
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

function formatFileMeta(entry: FileEntry): string {
    return `${formatBytes(entry.size)} · ${entry.mode}`;
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

void new OpsyShell().bootstrap();
