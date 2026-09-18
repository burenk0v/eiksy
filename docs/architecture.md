# Eiksy Architecture

Eiksy is a cross-platform AI-powered remote operations workstation. The architecture is built around one principle:

> **AI may assist with infrastructure operations, but it is never the security boundary.**

The backend owns authorization, capabilities, credentials, execution and audit. The UI and AI consume explicit application services and domain contracts.

This document is the permanent architecture reference for the project. It describes the current design, constraints and extension rules. It intentionally does not describe development phases or temporary implementation steps.

## 1. Architectural goals

Eiksy combines remote connections and sessions, interactive terminal access, SFTP and remote file operations, RDP where supported, AI assistance and diagnostics, controlled AI-assisted command execution, multiple AI providers and credential providers.

The primary goals are:

1. Keep security-sensitive decisions in the backend.
2. Keep credentials outside ordinary application state and AI context.
3. Provide one controlled path for command execution.
4. Keep transport and AI provider implementations behind stable domain contracts.
5. Centralize security-relevant audit behavior.
6. Allow new features without repeatedly redesigning the application architecture.
7. Keep domain contracts small, explicit and transport/provider agnostic.

## 2. High-level architecture

```text
┌──────────────────────────────────────────────────────────────┐
│                         Eiksy UI                             │
│              Wails / frontend / user interaction             │
└──────────────────────────────┬───────────────────────────────┘
                               │ service API
                               ▼
┌──────────────────────────────────────────────────────────────┐
│                    Application Services                     │
│       orchestration / session / AI / file / auth flows       │
└───────────────┬───────────────────┬──────────────────────────┘
                │                   │
                ▼                   ▼
┌───────────────────────┐   ┌─────────────────────────────────┐
│   Domain Contracts    │   │     Security / Policy Layer     │
│ Connection / AI       │   │ capabilities / policy /        │
│ Provider / operations │   │ approval / human-in-the-loop    │
└───────────┬───────────┘   └───────────────┬─────────────────┘
            │                               │
            └───────────────┬───────────────┘
                            ▼
                 ┌──────────────────────┐
                 │      Executors       │
                 │ command / file /    │
                 │ connection actions  │
                 └──────────┬───────────┘
                            │
          ┌─────────────────┼──────────────────┐
          ▼                 ▼                  ▼
     SSH / SFTP            RDP              Filesystem

                 ┌──────────────────────┐
                 │    Secure Storage    │
                 │ credentials / tokens │
                 └──────────────────────┘

                 ┌──────────────────────┐
                 │       Audit          │
                 │ security-relevant    │
                 │ operations/events    │
                 └──────────────────────┘
```

The diagram is logical rather than a strict package dependency graph. Some domain contracts intentionally exist ahead of broad concrete runtime adoption. They define stable extension boundaries; they should not be forced into every implementation until a real second implementation or capability requires them.

The important property is the direction of authority: UI and AI request operations; backend policy and capability checks decide what may happen; executors perform the operation; audit records the lifecycle.

## 3. Core boundaries

### 3.1 UI boundary

The UI is a presentation and interaction layer, not a security boundary. UI code must not directly access credential storage, transport implementations, command executors, provider-specific authentication or filesystem security primitives.

Authorization and secret-handling decisions must remain enforceable when the UI is bypassed or modified.

### 3.2 Session boundary

A session represents user-facing connection configuration and lifecycle state. It may contain connection metadata required to identify and operate a session, but secret material belongs to secure storage.

Session state is not a credential vault.

Examples of ordinary session data include host, port, username, protocol, connection options, provider/path identifiers, history and presentation metadata. Passwords, private keys, passphrases, AI API tokens and decrypted provider values belong in secure storage.

The Wails input DTO may temporarily carry credential material because credentials have to cross the UI-to-backend request boundary. The application boundary must move those values directly into secure storage and must not expose them as ordinary session state. Stored/session-facing `Profile` objects must remain secret-free.

### 3.3 Connection boundary

`internal/domain/connections` provides the transport-neutral connection contract. Transport-specific details remain below the domain boundary.

Current and planned transports can include SSH, SFTP and RDP.

**Rule:** adding a transport means implementing the appropriate connection or capability contract; it does not mean adding transport-specific branches throughout the application.

### 3.4 Command execution boundary

Command execution is a capability, not a convenience method exposed by every feature.

```text
AI / user operation
        ↓
Command Policy
        ↓
Capability / authorization checks
        ↓
Approval when required
        ↓
Command Executor
        ↓
Remote execution
        ↓
Bounded result
        ↓
Audit
```

There must not be a second direct path from AI, diagnostics, remediation or another feature to SSH command execution.

### 3.5 AI boundary

AI-specific architecture, context, native tools, command execution, diagnostics, remediation, security invariants and extension rules are documented in [`docs/ai.md`](ai.md).

AI does not own authorization, credentials or transport access.

### 3.6 Secure storage boundary

Credentials are application-owned secrets. Secure storage is the normal persistence boundary for sensitive credential material.

```text
configuration / metadata → ordinary application state
secret value             → secure storage
```

Credential providers must not become secret browsers for the UI or AI.

### 3.7 Filesystem and remote file boundary

Local and remote file operations are capabilities with explicit validation and bounded data flow. SFTP operations remain behind the connection/operation boundary. Local filesystem operations must not become an unintended arbitrary-file access path for AI operations.

File content may be sensitive even when it is not a credential, so file reads/searches can require approval according to policy.

### 3.8 Audit boundary

Security-relevant operations use the centralized audit path rather than feature-specific logs that can bypass redaction or lifecycle tracking.

The current command audit implementation is a **bounded local operational trail**. It is persisted by the disk store when available and remains bounded by count and field size. It is not a compliance/SIEM audit backend.

Audit data must not contain raw passwords, API tokens, private keys, secret values or other authentication material.

## 4. Data ownership model

| Data | Owner | Normal persistence | AI visibility |
| --- | --- | --- | --- |
| Session metadata | Application/session layer | Workspace/session state | Relevant metadata only |
| Connection options | Connection/application layer | Workspace/session state | Non-secret context only |
| Passwords | Secure storage | Secure storage | Never |
| SSH private material | Secure storage | Secure storage | Never |
| SSH passphrases | Secure storage | Secure storage | Never |
| AI provider tokens | Secure storage | Secure storage | Never |
| Vault/KeePass secret values | Credential provider / secure runtime | Provider storage | Never |
| Provider/path metadata | Application configuration | Workspace state | Only when operationally useful |
| Infrastructure facts | AI context builder | Runtime only unless explicitly persisted | Allowed, subject to sensitivity rules |
| Command text | Execution layer | Redacted/bounded audit where applicable | Allowed subject to policy |
| Command output | Executor/result layer | Bounded/redacted audit where applicable | Bounded |
| Errors | Application/execution layer | Bounded/redacted audit where applicable | Bounded |

## 5. Advantages

### Security by construction

Sensitive operations have a small number of controlled entry points, making authorization, credential exposure and audit coverage easier to reason about.

### Human-in-the-loop AI

AI reasoning is separated from authorization. An AI provider can suggest an action without receiving unrestricted infrastructure authority.

### Provider and transport independence

Cloud/local AI and SSH/SFTP/RDP can evolve behind explicit contracts without leaking implementation details into unrelated code.

### Testability

Small contracts and explicit boundaries allow security invariants to be tested without exercising the entire desktop application.

### Incremental development

Features can be added on top of stable capabilities instead of creating another application-wide architecture layer for each feature.

## 6. Trade-offs

### More indirection

Security-sensitive operations pass through service, policy and executor boundaries instead of calling a transport directly.

**Trade-off:** more code at the boundary in exchange for centralized security behavior.

### Limited AI visibility

AI cannot freely inspect credentials, arbitrary files or unrestricted command output.

**Trade-off:** some diagnostics require targeted tools or approval, but AI is not treated as a trusted secret-handling component.

### Centralized execution

Features cannot introduce convenient one-off execution helpers.

**Trade-off:** feature work must fit the existing execution model, preventing command security from fragmenting.

### Bounded results

Command output and errors are bounded before downstream AI processing.

**Trade-off:** complex investigations may require targeted commands or dedicated diagnostic tools.

## 7. Extension rules

1. **UI → application service only.**
2. **New transports → connection/operation contracts.**
3. **New AI backends → AI provider contract.**
4. **Commands → existing policy/capability/executor pipeline.**
5. **Credentials → secure storage.**
6. **AI context → runtime operational facts without secrets.**
7. **Security-relevant operations → centralized audit.**
8. **Security-sensitive behavior → regression tests.**
9. **Feature work must not create parallel storage, credential, transport or command paths.**
10. **Architecture changes require a concrete security, correctness, scalability or platform requirement.**

## 8. Architecture change policy

The architecture is considered stable.

Revisit it only when there is a concrete requirement that cannot be satisfied safely and correctly within the existing boundaries, such as a demonstrated security flaw, correctness problem, scalability limitation, required platform capability or fundamental change in the product's execution model.

A new feature, by itself, is not sufficient justification for an architectural rewrite.

> **Build features, not foundations.**
