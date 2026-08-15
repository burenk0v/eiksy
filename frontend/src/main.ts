import './style.css';
import './app.css';

import {CloseSession, GetShellState, LaunchSession} from '../wailsjs/go/main/App';
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
        <article class="provider-card">
            <div>
                <strong>${escapeHtml(provider.name)}</strong>
                <div class="session-meta">${escapeHtml(provider.class)} · ${escapeHtml(provider.model)}</div>
            </div>
            <div class="provider-capabilities">${provider.configured ? 'configured' : 'token required'}</div>
        </article>
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
