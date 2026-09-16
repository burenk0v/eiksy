# Eiksy Architecture

Eiksy is a cross-platform AI-powered remote operations workstation. The architecture is built around one principle:

> **AI may assist with infrastructure operations, but it is never the security boundary.**

The backend owns authorization, capabilities, credentials, execution and audit. The UI and AI consume explicit application services and domain contracts.

This document is the permanent architecture reference for the project. It describes the current design, its constraints, and the reasons for the main boundaries. It intentionally does not describe development phases, milestones, or temporary implementation steps.

## 1. Architectural goals

Eiksy combines:

- remote connections and sessions;
- interactive terminal access;
- SFTP and remote file operations;
- RDP where supported;
- AI assistance and infrastructure diagnostics;
- controlled AI-assisted command execution;
- multiple AI providers;
- multiple credential providers;
- centralized operational audit.

### Primary goals

1. Keep security-sensitive decisions in the backend.
2. Keep credentials outside ordinary application state and AI context.
3. Provide one controlled path for command execution.
4. Keep transport and AI provider implementations behind stable domain contracts.
5. Make audit a cross-cutting operational concern rather than a feature-specific implementation.
6. Allow new product features without repeatedly redesigning the application architecture.
7. Keep the domain contracts small, explicit and transport/provider agnostic.

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
│                       │   │                                 │
│ Connection            │   │ capabilities                    │
│ AI Provider           │   │ command policy                  │
│ operation contracts   │   │ approval / human-in-the-loop    │
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
          │
          └─────────────────┬─────────────────┘
                            ▼
                 ┌──────────────────────┐
                 │    Secure Storage    │
                 │ credentials / tokens │
                 └──────────────────────┘

                 ┌──────────────────────┐
                 │    Central Audit     │
                 │ operations / result  │
                 │ security-relevant    │
                 │ events               │
                 └──────────────────────┘
```

The diagram is logical rather than a strict package dependency graph. The important property is the direction of authority: UI and AI request operations; backend policy and capability checks decide what may happen; executors perform the operation; audit records the lifecycle.

## 3. Core boundaries

### 3.1 UI boundary

The UI is a presentation and interaction layer, not a security boundary.

UI code communicates with backend application services. It must not directly access:

- credential storage;
- transport implementations;
- command executors;
- provider-specific authentication;
- filesystem security primitives.

Wails DTOs are ordinary application data contracts. They must not become a covert channel for credentials or other security-sensitive runtime state.

**Constraint:** authorization and secret-handling decisions must remain enforceable when the UI is bypassed or modified.

### 3.2 Session boundary

A session represents user-facing connection configuration and lifecycle state. It may contain connection metadata required to identify and operate a session, but secret material belongs to secure storage.

Session state is not a credential vault.

Examples of session/configuration data:

- host and port;
- username;
- protocol/transport selection;
- connection options;
- provider/path identifiers for credential lookup;
- session history and presentation metadata.

Examples of data that must not be persisted in ordinary session state:

- passwords;
- SSH private keys or passphrases;
- AI API tokens;
- decrypted Vault/KeePass values;
- other authentication secrets.

### 3.3 Connection boundary

`internal/domain/connections` provides the transport-neutral connection contract.

The connection abstraction represents connection lifecycle and common connection semantics without forcing transport-specific state into unrelated domain objects.

Current and planned transports can include:

- SSH;
- SFTP;
- RDP;
- local operations where applicable.

Transport-specific implementation details remain below the domain boundary.

**Rule:** adding a transport means implementing the connection contract or a clearly scoped capability contract; it does not mean adding transport-specific branches throughout the application.

### 3.4 Command execution boundary

Command execution is a capability, not a convenience method exposed by every feature.

The controlled execution path is:

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

There must not be a second direct path from UI, AI, diagnostics, remediation or another feature to SSH command execution.

The executor is responsible for performing an already-authorized operation. It must not be used as a mechanism to bypass policy.

### 3.5 AI boundary

AI-specific architecture, context, native tools, command execution, diagnostics, remediation, security invariants and extension rules are documented in [`docs/ai.md`](ai.md).

The application architecture treats AI as an explicit capability behind an application service and provider contract. AI does not own authorization, credentials or transport access.

### 3.6 Secure storage boundary

Credentials are application-owned secrets.

Secure storage is the only normal persistence boundary for sensitive credential material. Depending on the platform and provider, this can include OS-backed secure storage and application-side integrations with providers such as Vault or KeePass.

The important invariant is independent of the backend implementation:

```text
configuration / metadata → ordinary application state
secret value             → secure storage
```

Credential providers must not become secret browsers for the UI or AI.

### 3.7 Vault / KeePass boundary

Vault and KeePass are application-side credential sources.

The user-facing model is based on an explicit provider and secret path rather than exposing a general-purpose secret tree to the UI.

A narrow credential-path validation operation may:

1. validate the selected provider;
2. normalize and validate the path;
3. reject empty paths, NUL bytes and traversal outside the provider root;
4. query only the information required to validate the requested leaf;
5. return success/error without returning the secret value.

The intended flow is:

```text
Vault / KeePass
      ↓
backend
      ↓
explicit connection operation
```

not:

```text
Vault / KeePass
      ↓
UI / AI
      ↓
secret value
      ↓
connection
```

### 3.8 Filesystem boundary

Local and remote file operations are capabilities with explicit validation and bounded data flow.

Remote SFTP operations remain behind the connection/operation boundary. Local filesystem operations must validate transfer targets and must not be used to create an unintended arbitrary-file access path for AI operations.

File content may be sensitive even when it is not formally a credential. Therefore ordinary file reads/searches can require approval according to policy.

### 3.9 Audit boundary

Audit is centralized.

Operations that participate in the operational audit model must use the same audit path rather than creating feature-specific logs that can bypass security redaction or lifecycle tracking.

Audit data must not contain raw:

- passwords;
- API tokens;
- private keys;
- secret values;
- other authentication material.

Command, error and result data are subject to the same redaction and bounded-output rules as the rest of the execution pipeline.

## 4. Data ownership model

| Data | Owner | Normal persistence | AI visibility |
| --- | --- | --- | --- |
| Session metadata | Application/session layer | Workspace/session state | Relevant metadata only |
| Connection options | Connection/application layer | Workspace/session state | Only non-secret context |
| Passwords | Secure storage | Secure storage | Never |
| SSH private material | Secure storage | Secure storage | Never |
| SSH passphrases | Secure storage | Secure storage | Never |
| AI provider tokens | Secure storage | Secure storage | Never |
| Vault/KeePass secret values | Credential provider / secure runtime | Provider storage | Never |
| Provider/path metadata | Application configuration | Workspace state | Only when operationally useful |
| Infrastructure facts | AI context builder | Runtime only unless explicitly persisted | Allowed, subject to sensitivity rules |
| Command text | Execution layer | Audit in redacted/bounded form | Allowed subject to policy |
| Command output | Executor/result layer | Audit in bounded/redacted form where applicable | Bounded |
| Errors | Application/execution layer | Audit in bounded/redacted form | Bounded |

## 5. Architectural advantages

### Security by construction

Sensitive operations have a small number of controlled entry points. This makes it possible to reason about authorization, credential exposure and audit coverage without inspecting every feature independently.

### Human-in-the-loop AI

The architecture separates AI reasoning from authorization. An AI provider can suggest an action without receiving unrestricted infrastructure authority.

### Provider independence

AI and credential providers can evolve independently of the UI and core domain contracts. Cloud and local AI can use the same application workflow.

### Transport independence

SSH, SFTP, RDP and future transports can share application concepts without forcing their implementation details into the domain model.

### Testability

Small contracts and explicit boundaries allow security invariants to be tested without requiring every test to exercise the full desktop application.

### Incremental product development

Features can be added on top of stable capabilities rather than creating another application-wide architecture layer for each feature.

### Auditability

A centralized operation path makes it possible to reconstruct what was requested, what policy decided, what was approved, what executed and what result was returned, while applying consistent redaction.

## 6. Architectural constraints and trade-offs

The architecture intentionally accepts some constraints in exchange for security and maintainability.

### More indirection

A simple operation may pass through service, policy, capability and executor layers instead of calling a transport directly.

**Trade-off:** more code and concepts at the boundary; in return, security-sensitive behavior is centralized and reusable.

### Limited AI visibility

AI cannot freely inspect credentials, environment variables, arbitrary files or unrestricted command output.

**Trade-off:** some diagnostics require explicit operations and may need human approval; in return, AI cannot be treated as a trusted secret-handling component.

### Centralized execution

Features cannot introduce convenient one-off execution helpers.

**Trade-off:** feature implementation must fit the existing executor and policy model; in return, command security does not fragment over time.

### Narrow provider contracts

Provider abstractions intentionally expose only the behavior required by the application.

**Trade-off:** provider-specific advanced functionality may require an explicit capability or extension rather than leaking provider details into common code.

### Explicit approval

Some operations require a human decision even when AI can technically perform them.

**Trade-off:** automation is not always one-click; in return, the operator remains the authority for sensitive actions.

### Bounded results

Command stdout, stderr and errors are bounded before they are returned to AI or UI paths.

**Trade-off:** a diagnostic may need pagination or a more targeted command; in return, context growth and accidental data exposure are constrained.

## 7. Extension rules

Future functionality should follow these rules.

1. **UI → application service only.**
2. **New transports → connection/operation contracts.**
3. **New AI backends → AI provider contract.**
4. **Commands → existing policy/capability/executor pipeline.**
5. **Credentials → secure storage.**
6. **AI context → runtime operational facts without secrets.**
7. **Operations → centralized audit where applicable.**
8. **Security-sensitive behavior → regression tests.**
9. **Feature work must not create parallel storage, credential, transport or command paths.**
10. **Architecture changes require a concrete security, correctness, scalability or platform requirement.**

A feature may add a new capability, service, adapter or provider implementation when the existing boundaries require it. It should not introduce a second architecture merely because that is locally more convenient.

## 8. Architecture change policy

The architecture is considered stable.

Revisit it only when there is a concrete requirement that cannot be satisfied safely and correctly within the existing boundaries, such as:

- a demonstrated security flaw;
- a correctness problem caused by an existing boundary;
- a scalability limitation that cannot be addressed locally;
- a required platform capability incompatible with the current design;
- a fundamental change in the product's execution model.

A new feature, by itself, is not sufficient justification for an architectural rewrite.

> **Build features, not foundations.**
