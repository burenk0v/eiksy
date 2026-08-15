import './style.css';
import './app.css';

import {
    CloseSession,
    DownloadLocalModel,
    GetShellState,
    LaunchSession,
    SaveCloudProvider,
    SelectAIProvider,
    StartLocalModel,
} from '../wailsjs/go/main/App';
import type {ai as aiModels, app as appModels, credentials, protocols, sessions} from '../wailsjs/go/models';

type ProtocolDescriptor = protocols.Descriptor;
type SessionProfile = sessions.Profile;
type RuntimeSession = appModels.RuntimeSessionView;
type CredentialProvider = credentials.ProviderDescriptor;
type AIProvider = aiModels.ProviderDescriptor;
type ShellState = appModels.ShellState;

const app = document.querySelector<HTMLDivElement>('#app');

async function bootstrap(transientError = '') {
    if (!app) {
        return;
    }

    const state: ShellState = await GetShellState();
    render(state, transientError);
}

function render(state: ShellState, transientError = '') {
    if (!app) {
        return;
    }

    app.innerHTML = `
        <div class="shell">
            <aside class="panel sidebar">
                <div class="panel-header">
                    <div>
                        <div class="eyebrow">Session manager</div>
                        <h1>opsy</h1>
                    </div>
                    <div class="pill">${escapeHtml(state.settings.defaultProtocol.toUpperCase())} default</div>
                </div>

                <section class="section">
                    <div class="section-title">Saved sessions</div>
                    <div class="session-list">
                        ${state.sessionProfiles.map(renderProfile).join('')}
                    </div>
                </section>

                <section class="section">
                    <div class="section-title">Credential providers</div>
                    <div class="provider-list">
                        ${state.credentialProviders.map(renderProvider).join('')}
                    </div>
                </section>
            </aside>

            <main class="panel workspace">
                <div class="panel-header">
                    <div>
                        <div class="eyebrow">Workspace</div>
                        <h2>Active tabs</h2>
                    </div>
                    <div class="tab-count">${state.activeSessions.length} open</div>
                </div>

                ${transientError ? `<div class="error-banner">${escapeHtml(transientError)}</div>` : ''}

                <div class="tabs">
                    ${state.activeSessions.map(renderTab).join('') || '<div class="empty-state">No active sessions</div>'}
                </div>

                <div class="workspace-grid">
                    <section class="card">
                        <div class="section-title">Protocols</div>
                        <ul class="tag-list">
                            ${state.protocols.map(renderProtocol).join('')}
                        </ul>
                    </section>

                    <section class="card">
                        <div class="section-title">Recent launches</div>
                        <ul class="activity-list">
                            ${state.sessionHistory.map((entry) => `<li><strong>${escapeHtml(entry.profileName)}</strong><span>${escapeHtml(formatDate(entry.launchedAt))}</span></li>`).join('')}
                        </ul>
                    </section>

                    <section class="card full-width">
                        <div class="section-title">Backend events</div>
                        <ul class="activity-list">
                            ${state.workspace.recentEvents.map((event) => `<li><strong>${escapeHtml(event.type)}</strong><span>${escapeHtml(event.subject)} · ${escapeHtml(formatDate(event.at))}</span></li>`).join('')}
                        </ul>
                    </section>
                </div>
            </main>

            <aside class="panel assistant">
                <div class="panel-header">
                    <div>
                        <div class="eyebrow">AI assistant</div>
                        <h2>Command help</h2>
                    </div>
                    <div class="pill">${state.ai.contextPolicy.requireConfirmation ? 'guarded' : 'open'}</div>
                </div>

                <section class="section">
                    <div class="section-title">Providers</div>
                    <div class="provider-list">
                        ${state.ai.providers.map(renderAIProvider).join('')}
                    </div>
                </section>

                <section class="section">
                    <div class="section-title">Provider setup</div>
                    ${renderAIProviderSetup(state.ai.providers)}
                </section>

                <section class="section">
                    <div class="section-title">Context policy</div>
                    <ul class="tag-list">
                        <li>${escapeHtml(state.ai.contextPolicy.sendTerminalSelection ? 'Selection sharing enabled' : 'Selection sharing disabled')}</li>
                        <li>${escapeHtml(state.ai.contextPolicy.sendRecentOutput ? 'Recent output sharing enabled' : 'Recent output sharing disabled')}</li>
                        <li>${escapeHtml(state.settings.allowCloudModels ? 'Cloud models allowed' : 'Cloud models disabled')}</li>
                    </ul>
                </section>

                <section class="section chat">
                    <div class="section-title">Assistant panel</div>
                ${state.ai.messages.map((message) => `<div class="message ${escapeClassName(message.role)}">${escapeHtml(message.content)}</div>`).join('')}
                </section>
            </aside>
        </div>
    `;

    app.querySelectorAll<HTMLButtonElement>('[data-open-profile]').forEach((button) => {
        button.addEventListener('click', async () => {
            const profileID = button.dataset.openProfile;
            if (!profileID) {
                return;
            }

            let nextError = '';
            try {
                await LaunchSession(profileID);
            } catch (error) {
                nextError = formatError('Unable to launch session', error);
            }

            await bootstrap(nextError);
        });
    });

    app.querySelectorAll<HTMLButtonElement>('[data-close-session]').forEach((button) => {
        button.addEventListener('click', async () => {
            const sessionID = button.dataset.closeSession;
            if (!sessionID) {
                return;
            }

            let nextError = '';
            try {
                await CloseSession(sessionID);
            } catch (error) {
                nextError = formatError('Unable to close session', error);
            }

            await bootstrap(nextError);
        });
    });

    app.querySelectorAll<HTMLButtonElement>('[data-select-ai-provider]').forEach((button) => {
        button.addEventListener('click', async () => {
            const providerID = button.dataset.selectAiProvider;
            if (!providerID) {
                return;
            }

            let nextError = '';
            try {
                await SelectAIProvider(providerID);
            } catch (error) {
                nextError = formatError('Unable to switch AI provider', error);
            }

            await bootstrap(nextError);
        });
    });

    const downloadButton = app.querySelector<HTMLButtonElement>('[data-download-local-model]');
    downloadButton?.addEventListener('click', async () => {
        let nextError = '';
        try {
            await DownloadLocalModel();
        } catch (error) {
            nextError = formatError('Unable to download Qwen3 8B', error);
        }

        await bootstrap(nextError);
    });

    const startButton = app.querySelector<HTMLButtonElement>('[data-start-local-model]');
    startButton?.addEventListener('click', async () => {
        let nextError = '';
        try {
            await StartLocalModel();
        } catch (error) {
            nextError = formatError('Unable to start llama.cpp', error);
        }

        await bootstrap(nextError);
    });

    const cloudForm = app.querySelector<HTMLFormElement>('[data-cloud-provider-form]');
    cloudForm?.addEventListener('submit', async (event) => {
        event.preventDefault();

        const endpointInput = cloudForm.querySelector<HTMLInputElement>('input[name="endpoint"]');
        const tokenInput = cloudForm.querySelector<HTMLInputElement>('input[name="token"]');
        if (!endpointInput || !tokenInput) {
            return;
        }

        let nextError = '';
        try {
            await SaveCloudProvider(endpointInput.value, tokenInput.value);
        } catch (error) {
            nextError = formatError('Unable to save cloud model settings', error);
        }

        await bootstrap(nextError);
    });
}

function renderProfile(profile: SessionProfile) {
    return `
        <article class="session-card">
            <div>
                <div class="session-title-row">
                    <strong>${escapeHtml(profile.name)}</strong>
                    ${profile.favorite ? '<span class="favorite">★</span>' : ''}
                </div>
                <div class="session-meta">${escapeHtml(profile.group)} · ${escapeHtml(profile.protocolId.toUpperCase())} · ${escapeHtml(profile.username)}@${escapeHtml(profile.host)}:${escapeHtml(String(profile.port))}</div>
                <div class="session-tags">${profile.tags.map((tag) => `<span>${escapeHtml(tag)}</span>`).join('')}</div>
            </div>
            <button class="action-button" data-open-profile="${escapeHtml(profile.id)}">Open</button>
        </article>
    `;
}

function renderProvider(provider: CredentialProvider) {
    return `
        <article class="provider-card">
            <div>
                <strong>${escapeHtml(provider.name)}</strong>
                <div class="session-meta">${escapeHtml(provider.type)} · ${escapeHtml(provider.status.state)}</div>
            </div>
            <div class="provider-capabilities">${escapeHtml(provider.capabilities.join(', '))}</div>
        </article>
    `;
}

function renderTab(tab: RuntimeSession) {
    return `
        <article class="tab-card ${escapeClassName(tab.status)}">
            <div>
                <strong>${escapeHtml(tab.title)}</strong>
                <div class="session-meta">${escapeHtml(tab.description)}</div>
            </div>
            <button class="action-button secondary" data-close-session="${escapeHtml(tab.id)}">Close</button>
        </article>
    `;
}

function renderProtocol(protocol: ProtocolDescriptor) {
    return `<li><strong>${escapeHtml(protocol.scheme.toUpperCase())}</strong><span>${escapeHtml(protocol.capabilities.join(' · '))}</span></li>`;
}

function renderAIProvider(provider: AIProvider) {
    return `
        <article class="provider-card ${provider.selected ? 'selected-provider' : ''}">
            <div>
                <strong>${escapeHtml(provider.name)}</strong>
                <div class="session-meta">${escapeHtml(provider.class)} · ${escapeHtml(provider.model)}</div>
                ${provider.endpoint ? `<div class="provider-capabilities">${escapeHtml(provider.endpoint)}</div>` : ''}
            </div>
            <div class="provider-actions">
                <div class="provider-capabilities">${escapeHtml(provider.status)}</div>
                <button class="action-button secondary" data-select-ai-provider="${escapeHtml(provider.id)}">${provider.selected ? 'Selected' : 'Use'}</button>
            </div>
        </article>
    `;
}

function renderAIProviderSetup(providers: AIProvider[]) {
    const selectedProvider = providers.find((provider) => provider.selected);
    if (!selectedProvider) {
        return '<div class="empty-state">Select an AI provider to configure it.</div>';
    }

    if (selectedProvider.class === 'local') {
        return renderLocalProviderSetup(selectedProvider);
    }

    return renderCloudProviderSetup(selectedProvider);
}

function renderLocalProviderSetup(provider: AIProvider) {
    return `
        <div class="provider-setup-card">
            <div class="setup-copy">Download the bundled Qwen3 8B GGUF model, then run it through llama.cpp.</div>
            <ul class="detail-list">
                <li><strong>Model</strong><span>${escapeHtml(provider.model)}</span></li>
                <li><strong>Status</strong><span>${escapeHtml(provider.status)}</span></li>
                <li><strong>Path</strong><span>${escapeHtml(provider.localPath ?? 'Not downloaded yet')}</span></li>
                <li><strong>Endpoint</strong><span>${escapeHtml(provider.endpoint ?? 'Will be exposed after llama.cpp starts')}</span></li>
                <li><strong>Runner</strong><span>${escapeHtml(provider.command ?? 'llama-server must be available in PATH or OPSY_LLAMA_CPP_BIN')}</span></li>
            </ul>
            <div class="provider-form-actions">
                <button class="action-button" data-download-local-model>Download Qwen3 8B</button>
                <button class="action-button secondary" data-start-local-model ${provider.localPath ? '' : 'disabled'}>Start with llama.cpp</button>
            </div>
        </div>
    `;
}

function renderCloudProviderSetup(provider: AIProvider) {
    return `
        <form class="provider-form" data-cloud-provider-form>
            <label>
                <span>Endpoint URL</span>
                <input type="url" name="endpoint" value="${escapeHtml(provider.endpoint ?? '')}" placeholder="https://api.example.com/v1" required />
            </label>
            <label>
                <span>API token</span>
                <input type="password" name="token" placeholder="${provider.configured ? 'Enter a new token to replace the current one' : 'sk-...'}" required />
            </label>
            <div class="setup-copy">For cloud providers, specify the OpenAI-compatible base URL and token.</div>
            <div class="provider-form-actions">
                <button class="action-button" type="submit">Save cloud connection</button>
            </div>
        </form>
    `;
}

function formatDate(value: string) {
    const date = new Date(value);
    return Number.isNaN(date.getTime()) ? value : date.toLocaleString();
}

function escapeHtml(value: string) {
    return value
        .replaceAll('&', '&amp;')
        .replaceAll('<', '&lt;')
        .replaceAll('>', '&gt;')
        .replaceAll('"', '&quot;')
        .replaceAll("'", '&#39;');
}

function escapeClassName(value: string) {
    return value.replace(/[^a-zA-Z0-9_-]/g, '-');
}

function formatError(prefix: string, error: unknown) {
    if (error instanceof Error) {
        return `${prefix}: ${error.message}`;
    }

    return prefix;
}

void bootstrap();
