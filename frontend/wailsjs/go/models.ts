export namespace ai {
	
	export class ChatMessage {
	    role: string;
	    content: string;
	
	    static createFrom(source: any = {}) {
	        return new ChatMessage(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.role = source["role"];
	        this.content = source["content"];
	    }
	}
	export class ContextPolicy {
	    sendTerminalSelection: boolean;
	    sendRecentOutput: boolean;
	    requireConfirmation: boolean;
	
	    static createFrom(source: any = {}) {
	        return new ContextPolicy(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.sendTerminalSelection = source["sendTerminalSelection"];
	        this.sendRecentOutput = source["sendRecentOutput"];
	        this.requireConfirmation = source["requireConfirmation"];
	    }
	}
	export class ProviderDescriptor {
	    id: string;
	    name: string;
	    class: string;
	    model: string;
	    endpoint?: string;
	    localPath?: string;
	    command?: string;
	    status: string;
	    selected: boolean;
	    configured: boolean;
	
	    static createFrom(source: any = {}) {
	        return new ProviderDescriptor(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.class = source["class"];
	        this.model = source["model"];
	        this.endpoint = source["endpoint"];
	        this.localPath = source["localPath"];
	        this.command = source["command"];
	        this.status = source["status"];
	        this.selected = source["selected"];
	        this.configured = source["configured"];
	    }
	}
	export class WorkspaceState {
	    providers: ProviderDescriptor[];
	    contextPolicy: ContextPolicy;
	    messages: ChatMessage[];
	    chatSessionId: string;
	
	    static createFrom(source: any = {}) {
	        return new WorkspaceState(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.providers = this.convertValues(source["providers"], ProviderDescriptor);
	        this.contextPolicy = this.convertValues(source["contextPolicy"], ContextPolicy);
	        this.messages = this.convertValues(source["messages"], ChatMessage);
	        this.chatSessionId = source["chatSessionId"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}

}

export namespace app {
	
	export class RuntimeSessionView {
	    id: string;
	    title: string;
	    protocolId: string;
	    profileId: string;
	    status: string;
	    description: string;
	
	    static createFrom(source: any = {}) {
	        return new RuntimeSessionView(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.title = source["title"];
	        this.protocolId = source["protocolId"];
	        this.profileId = source["profileId"];
	        this.status = source["status"];
	        this.description = source["description"];
	    }
	}
	export class WorkspaceView {
	    layout: workspace.Layout;
	    recentEvents: workspace.Event[];
	
	    static createFrom(source: any = {}) {
	        return new WorkspaceView(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.layout = this.convertValues(source["layout"], workspace.Layout);
	        this.recentEvents = this.convertValues(source["recentEvents"], workspace.Event);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class ShellState {
	    protocols: protocols.Descriptor[];
	    sessionProfiles: sessions.Profile[];
	    activeSessions: RuntimeSessionView[];
	    sessionHistory: sessions.HistoryEntry[];
	    credentialProviders: credentials.ProviderDescriptor[];
	    ai: ai.WorkspaceState;
	    workspace: WorkspaceView;
	    settings: settings.AppSettings;
	
	    static createFrom(source: any = {}) {
	        return new ShellState(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.protocols = this.convertValues(source["protocols"], protocols.Descriptor);
	        this.sessionProfiles = this.convertValues(source["sessionProfiles"], sessions.Profile);
	        this.activeSessions = this.convertValues(source["activeSessions"], RuntimeSessionView);
	        this.sessionHistory = this.convertValues(source["sessionHistory"], sessions.HistoryEntry);
	        this.credentialProviders = this.convertValues(source["credentialProviders"], credentials.ProviderDescriptor);
	        this.ai = this.convertValues(source["ai"], ai.WorkspaceState);
	        this.workspace = this.convertValues(source["workspace"], WorkspaceView);
	        this.settings = this.convertValues(source["settings"], settings.AppSettings);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}

}

export namespace credentials {
	
	export class AuthStatus {
	    state: string;
	    expiresAt?: string;
	    renewable: boolean;
	    authenticated: boolean;
	
	    static createFrom(source: any = {}) {
	        return new AuthStatus(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.state = source["state"];
	        this.expiresAt = source["expiresAt"];
	        this.renewable = source["renewable"];
	        this.authenticated = source["authenticated"];
	    }
	}
	export class ProviderDescriptor {
	    id: string;
	    name: string;
	    type: string;
	    capabilities: string[];
	    status: AuthStatus;
	
	    static createFrom(source: any = {}) {
	        return new ProviderDescriptor(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.type = source["type"];
	        this.capabilities = source["capabilities"];
	        this.status = this.convertValues(source["status"], AuthStatus);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}

}

export namespace protocols {
	
	export class Descriptor {
	    id: string;
	    name: string;
	    scheme: string;
	    capabilities: string[];
	
	    static createFrom(source: any = {}) {
	        return new Descriptor(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.scheme = source["scheme"];
	        this.capabilities = source["capabilities"];
	    }
	}

}

export namespace sessions {
	
	export class HistoryEntry {
	    profileId: string;
	    profileName: string;
	    launchedAt: string;
	
	    static createFrom(source: any = {}) {
	        return new HistoryEntry(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.profileId = source["profileId"];
	        this.profileName = source["profileName"];
	        this.launchedAt = source["launchedAt"];
	    }
	}
	export class Profile {
	    id: string;
	    name: string;
	    group: string;
	    tags: string[];
	    favorite: boolean;
	    protocolId: string;
	    host: string;
	    port: number;
	    username: string;
	    password?: string;
	    secretRef?: string;
	    options?: Record<string, string>;
	    lastLaunchedAt?: string;
	
	    static createFrom(source: any = {}) {
	        return new Profile(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.group = source["group"];
	        this.tags = source["tags"];
	        this.favorite = source["favorite"];
	        this.protocolId = source["protocolId"];
	        this.host = source["host"];
	        this.port = source["port"];
	        this.username = source["username"];
	        this.password = source["password"];
	        this.secretRef = source["secretRef"];
	        this.options = source["options"];
	        this.lastLaunchedAt = source["lastLaunchedAt"];
	    }
	}
	export class ProfileInput {
	    id: string;
	    name: string;
	    group: string;
	    tags: string[];
	    favorite: boolean;
	    protocolId: string;
	    host: string;
	    port: number;
	    username: string;
	    password?: string;
	    secretRef?: string;
	    options?: Record<string, string>;
	    lastLaunchedAt?: string;
	
	    static createFrom(source: any = {}) {
	        return new ProfileInput(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.group = source["group"];
	        this.tags = source["tags"];
	        this.favorite = source["favorite"];
	        this.protocolId = source["protocolId"];
	        this.host = source["host"];
	        this.port = source["port"];
	        this.username = source["username"];
	        this.password = source["password"];
	        this.secretRef = source["secretRef"];
	        this.options = source["options"];
	        this.lastLaunchedAt = source["lastLaunchedAt"];
	    }
	}

}

export namespace settings {
	
	export class WindowLayout {
	    sidebarWidth: number;
	    assistantWidth: number;
	
	    static createFrom(source: any = {}) {
	        return new WindowLayout(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.sidebarWidth = source["sidebarWidth"];
	        this.assistantWidth = source["assistantWidth"];
	    }
	}
	export class AppSettings {
	    theme: string;
	    defaultProtocol: string;
	    windowLayout: WindowLayout;
	    promptBeforeAi: boolean;
	    allowCloudModels: boolean;
	    sshForwardPorts: string;
	    sshForwardHostId: string;
	    logLevel: string;
	    showLogPanel: boolean;
	    saveLogsToFile: boolean;
	    logRotationSize: number;
	    vaultAddress: string;
	    vaultMountPoint: string;
	    vaultToken: string;
	
	    static createFrom(source: any = {}) {
	        return new AppSettings(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.theme = source["theme"];
	        this.defaultProtocol = source["defaultProtocol"];
	        this.windowLayout = this.convertValues(source["windowLayout"], WindowLayout);
	        this.promptBeforeAi = source["promptBeforeAi"];
	        this.allowCloudModels = source["allowCloudModels"];
	        this.sshForwardPorts = source["sshForwardPorts"];
	        this.sshForwardHostId = source["sshForwardHostId"];
	        this.logLevel = source["logLevel"];
	        this.showLogPanel = source["showLogPanel"];
	        this.saveLogsToFile = source["saveLogsToFile"];
	        this.logRotationSize = source["logRotationSize"];
	        this.vaultAddress = source["vaultAddress"];
	        this.vaultMountPoint = source["vaultMountPoint"];
	        this.vaultToken = source["vaultToken"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}

}

export namespace sftp {
	
	export class FileEntry {
	    name: string;
	    path: string;
	    isDir: boolean;
	    size: number;
	    modTime: string;
	    mode: string;
	
	    static createFrom(source: any = {}) {
	        return new FileEntry(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.path = source["path"];
	        this.isDir = source["isDir"];
	        this.size = source["size"];
	        this.modTime = source["modTime"];
	        this.mode = source["mode"];
	    }
	}

}

export namespace workspace {
	
	export class Event {
	    id: string;
	    type: string;
	    subject: string;
	    at: string;
	
	    static createFrom(source: any = {}) {
	        return new Event(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.type = source["type"];
	        this.subject = source["subject"];
	        this.at = source["at"];
	    }
	}
	export class SidebarSection {
	    id: string;
	    title: string;
	
	    static createFrom(source: any = {}) {
	        return new SidebarSection(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.title = source["title"];
	    }
	}
	export class Layout {
	    sidebarSections: SidebarSection[];
	    activeTabId?: string;
	
	    static createFrom(source: any = {}) {
	        return new Layout(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.sidebarSections = this.convertValues(source["sidebarSections"], SidebarSection);
	        this.activeTabId = source["activeTabId"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}

}

export namespace vault {
	
	export class SecretNode {
	    name: string;
	    path: string;
	    isDir: boolean;
	
	    static createFrom(source: any = {}) {
	        return new SecretNode(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.path = source["path"];
	        this.isDir = source["isDir"];
	    }
	}

}
