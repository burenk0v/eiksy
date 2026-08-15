import './style.css';
import './app.css';

import {CloseSession, GetShellState, LaunchSession} from '../wailsjs/go/main/App';

type ProtocolDescriptor = {
    id: string;
    name: string;
    scheme: string;
    capabilities: string[];
};

type SessionProfile = {
    id: string;
    name: string;
    group: string;
    tags: string[];
    favorite: boolean;
    protocolId: string;
    host: string;
    port: number;
    username: string;
    secretRef?: string;
    lastLaunchedAt?: string;
};

type RuntimeSession = {
    id: string;
    title: string;
    protocolId: string;
    profileId: string;
    status: string;
    description: string;
};

type CredentialProvider = {
    id: string;
    name: string;
    type: string;
    capabilities: string[];
    status: {
        state: string;
        expiresAt?: string;
        renewable: boolean;
        authenticated: boolean;
    };
};

type AIProvider = {
    id: string;
    name: string;
    class: string;
    model: string;
    endpoint?: string;
    configured: boolean;
};

type ShellState = {
    protocols: ProtocolDescriptor[];
    sessionProfiles: SessionProfile[];
    activeSessions: RuntimeSession[];
    sessionHistory: Array<{profileId: string; profileName: string; launchedAt: string;}>;
    credentialProviders: CredentialProvider[];
    ai: {
        providers: AIProvider[];
        contextPolicy: {
            sendTerminalSelection: boolean;
            sendRecentOutput: boolean;
            requireConfirmation: boolean;
        };
        messages: Array<{role: string; content: string;}>;
    };
    workspace: {
        layout: {
            sidebarSections: Array<{id: string; title: string;}>;
            activeTabId?: string;
        };
        recentEvents: Array<{id: string; type: string; subject: string; at: string;}>;
    };
    settings: {
        theme: string;
        defaultProtocol: string;
        windowLayout: {
            sidebarWidth: number;
            assistantWidth: number;
        };
        promptBeforeAi: boolean;
        allowCloudModels: boolean;
    };
};

const app = document.querySelector<HTMLDivElement>('#app');

async function bootstrap() {
    if (!app) {
        return;
    }

    const state = await GetShellState() as ShellState;
    render(state);
}

function render(state: ShellState) {
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
                    <div class="pill">${state.settings.defaultProtocol.toUpperCase()} default</div>
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
                            ${state.sessionHistory.map((entry) => `<li><strong>${entry.profileName}</strong><span>${formatDate(entry.launchedAt)}</span></li>`).join('')}
                        </ul>
                    </section>

                    <section class="card full-width">
                        <div class="section-title">Backend events</div>
                        <ul class="activity-list">
                            ${state.workspace.recentEvents.map((event) => `<li><strong>${event.type}</strong><span>${event.subject} · ${formatDate(event.at)}</span></li>`).join('')}
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
                        <li>${state.ai.contextPolicy.sendTerminalSelection ? 'Selection sharing enabled' : 'Selection sharing disabled'}</li>
                        <li>${state.ai.contextPolicy.sendRecentOutput ? 'Recent output sharing enabled' : 'Recent output sharing disabled'}</li>
                        <li>${state.settings.allowCloudModels ? 'Cloud models allowed' : 'Cloud models disabled'}</li>
                    </ul>
                </section>

                <section class="section chat">
                    <div class="section-title">Assistant panel</div>
                    ${state.ai.messages.map((message) => `<div class="message ${message.role}">${message.content}</div>`).join('')}
                </section>
            </aside>
        </div>
    `;

    for (const button of app.querySelectorAll<HTMLButtonElement>('[data-open-profile]')) {
        button.addEventListener('click', async () => {
            const profileID = button.dataset.openProfile;
            if (!profileID) {
                return;
            }

            await LaunchSession(profileID);
            await bootstrap();
        });
    }

    for (const button of app.querySelectorAll<HTMLButtonElement>('[data-close-session]')) {
        button.addEventListener('click', async () => {
            const sessionID = button.dataset.closeSession;
            if (!sessionID) {
                return;
            }

            await CloseSession(sessionID);
            await bootstrap();
        });
    }
}

function renderProfile(profile: SessionProfile) {
    return `
        <article class="session-card">
            <div>
                <div class="session-title-row">
                    <strong>${profile.name}</strong>
                    ${profile.favorite ? '<span class="favorite">★</span>' : ''}
                </div>
                <div class="session-meta">${profile.group} · ${profile.protocolId.toUpperCase()} · ${profile.username}@${profile.host}:${profile.port}</div>
                <div class="session-tags">${profile.tags.map((tag) => `<span>${tag}</span>`).join('')}</div>
            </div>
            <button class="action-button" data-open-profile="${profile.id}">Open</button>
        </article>
    `;
}

function renderProvider(provider: CredentialProvider) {
    return `
        <article class="provider-card">
            <div>
                <strong>${provider.name}</strong>
                <div class="session-meta">${provider.type} · ${provider.status.state}</div>
            </div>
            <div class="provider-capabilities">${provider.capabilities.join(', ')}</div>
        </article>
    `;
}

function renderTab(tab: RuntimeSession) {
    return `
        <article class="tab-card ${tab.status}">
            <div>
                <strong>${tab.title}</strong>
                <div class="session-meta">${tab.description}</div>
            </div>
            <button class="action-button secondary" data-close-session="${tab.id}">Close</button>
        </article>
    `;
}

function renderProtocol(protocol: ProtocolDescriptor) {
    return `<li><strong>${protocol.scheme.toUpperCase()}</strong><span>${protocol.capabilities.join(' · ')}</span></li>`;
}

function renderAIProvider(provider: AIProvider) {
    return `
        <article class="provider-card">
            <div>
                <strong>${provider.name}</strong>
                <div class="session-meta">${provider.class} · ${provider.model}</div>
            </div>
            <div class="provider-capabilities">${provider.configured ? 'configured' : 'token required'}</div>
        </article>
    `;
}

function formatDate(value: string) {
    const date = new Date(value);
    return Number.isNaN(date.getTime()) ? value : date.toLocaleString();
}

void bootstrap();
