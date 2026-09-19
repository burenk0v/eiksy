# ADR-0001: CLI and TUI architecture

- Status: Accepted
- Date: 2026-09-19
- Scope: Eiksy CLI/TUI frontend
- Related: [architecture.md](../architecture.md), [ai.md](../ai.md), [product-contract.md](../product-contract.md)

## Context

Eiksy currently provides its user interaction surface through the Wails desktop UI. The product architecture deliberately keeps security-sensitive behavior in the backend application services, domain contracts, policy layer and executors.

Eiksy also needs a terminal-first CLI/TUI that can provide a workflow similar in spirit to modern AI coding and operations terminals: persistent tabs, AI conversations, tool activity, sessions and operational views.

The CLI must not create a second implementation of AI, sessions, command execution, credentials, policy, audit or transport behavior.

The CLI is an additional interaction surface for the existing Eiksy product.

## Decision

### 1. CLI is a frontend, not a second application

The CLI/TUI is a presentation and interaction layer over the existing Eiksy application services and domain contracts.

The intended dependency direction is:

```
                 Eiksy
                   |
          Application Services
                   |
          Domain / Security
                   |
        +----------+----------+
        |                     |
     Wails UI              CLI/TUI
```

Both frontends must use the same backend authorization, policy, credential, execution and audit boundaries.

The CLI must not call SSH/SFTP managers, secure storage implementations, AI provider implementations or command executors directly.

### 2. CLI has its own TUI state

The CLI needs presentation state that is not part of backend domain state.

Examples:

- active tab;
- focused pane;
- scroll position;
- command palette state;
- input buffer;
- terminal dimensions;
- selected session;
- visual expansion/collapse state.

This state belongs to the CLI/TUI layer.

It must not be persisted as domain state unless a concrete product requirement makes it part of a user-facing workspace contract.

### 3. Tabs represent UI workspaces

A CLI tab is a user-facing workspace containing references to the underlying Eiksy session/context.

Conceptually:

```
Tab
 |
 +-- UI state
 |
 +-- Session reference
 |
 +-- active view
```

A tab is not itself a connection, credential container or authorization boundary.

The first implementation will support multiple tabs while keeping session ownership and security decisions in the existing application layer.

### 4. Sessions remain application-owned

The existing Eiksy session model remains the source of truth for connection/session state.

The CLI may select, display and create sessions through application services, but it must not introduce an independent session database or connection registry. Persistent AI conversations are application-owned and their message content remains in secure storage.

Conversation state must follow the existing AI security rules and must never become a credential store.

### 5. Chat uses an event stream

The CLI chat will consume a normalized application/agent event stream rather than polling provider-specific state.

The target event model includes events such as:

- message started;
- text delta;
- tool started;
- tool output;
- tool finished;
- message finished;
- error;
- cancellation.

The exact Go types will be introduced when the agent event protocol is implemented.

The event protocol must remain independent of a specific LLM provider and a specific TUI framework.

### 6. TUI framework

The CLI will use the Charm ecosystem, centered on Bubble Tea, for terminal application lifecycle and event-driven rendering.

Supporting Charm components may be introduced only when needed for a concrete UI capability, such as styling or reusable terminal components.

The framework is an implementation detail of the CLI. It must not leak into application/domain contracts.

### 7. Views are composable

The TUI will be structured around explicit views/components rather than a single monolithic renderer.

Initial conceptual views:

```
App
 +-- Header / TabBar
 +-- MainView
 |    +-- Chat
 |    +-- Terminal
 |    +-- Files
 |    +-- other operational views
 +-- StatusBar
 +-- Input / CommandPalette
```

Not every view needs to exist in the first CLI release. The structure exists to prevent later features from turning the root model into a monolith.

### 8. Command palette and keyboard navigation are presentation concerns

Keyboard shortcuts, command palette behavior, focus and navigation belong to the TUI.

They must invoke application capabilities through explicit commands/actions rather than embedding business logic in key handlers.

### 9. Security boundary remains in the backend

The CLI is not a security boundary.

For an operation that requires approval:

```
CLI / AI
   ↓
Application service
   ↓
Policy / capability checks
   ↓
Approval
   ↓
Existing executor
   ↓
Audit
```

The CLI must not implement a local allow/deny mechanism that can bypass backend policy.

Approval UX may be different from the Wails UI, but the authorization decision must remain backend-enforceable.

### 10. No parallel storage

The CLI must not introduce independent storage for:

- credentials;
- connection profiles;
- command policy;
- audit records;
- AI provider tokens;
- AI conversation security state.
- persistent AI conversation storage outside the application-owned secure storage.

Persistent CLI-specific UI preferences may be added later as ordinary application configuration when justified.

## Consequences

### Positive

- Web UI and CLI share the same security model.
- Existing Eiksy capabilities can be reused instead of duplicated.
- AI behavior remains consistent across frontends.
- Security fixes in the backend protect every frontend.
- CLI-specific UX can evolve independently.
- Tabs and views can be implemented without changing the product's core security boundary.

### Trade-offs

- The CLI may need new application-service methods or contracts when an existing service does not expose a capability cleanly.
- Some UI-specific behavior requires an adapter between backend events and TUI events.
- The TUI introduces another frontend with its own testing requirements.
- The CLI cannot use shortcuts that bypass backend authorization for convenience.

## Extension rules

1. Reuse existing application services before adding new ones.
2. Do not call transport managers or secure storage directly from TUI code.
3. Do not duplicate AI provider logic in the CLI.
4. Do not duplicate command execution or policy evaluation.
5. Keep TUI framework types out of domain contracts.
6. Keep secrets out of TUI state, logs and rendered history.
7. Add regression tests for security-sensitive CLI behavior.
8. Prefer small vertical PRs: foundation, tabs, sessions, chat, agent events, tools and polish.
9. Add persistent CLI state only when it represents a real product requirement.
10. Revisit this architecture only when a concrete requirement cannot be satisfied within these boundaries.

## Initial implementation boundary

The first CLI PRs intentionally do not implement AI or infrastructure operations.

The sequence begins with:

1. CLI entrypoint;
2. TUI lifecycle and layout;
3. tabs;
4. session references;
5. chat presentation;
6. agent event protocol;
7. existing AI integration;
8. tool rendering and execution;
9. approval/security UX.

This keeps the architecture incremental and prevents the initial CLI work from creating parallel application infrastructure.

> **One Eiksy core, multiple interaction surfaces.**
