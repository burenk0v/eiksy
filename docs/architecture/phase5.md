# Phase 5 — Functional Platform v1

Phase 5 closes the architecture-foundation roadmap and establishes the product-facing contract for future Eiksy development.

## Goal

Eiksy should now be treated as a functional remote-operations workstation rather than an architecture exercise. New work should add user-visible capabilities on top of the existing boundaries instead of introducing another application-wide redesign.

## Current operator workflow

```text
Session
  ↓
Connect
  ↓
Operate
  ├── Terminal
  ├── SFTP
  └── RDP (where supported)
  ↓
Ask AI
  ↓
Infrastructure context
  ↓
Diagnostics / analysis
  ↓
Approval when required
  ↓
Controlled command execution
  ↓
Verify
  ↓
Audit
```

## Functional Platform v1

### Sessions

- saved connection profiles;
- multiple active sessions;
- SSH configuration import;
- session lifecycle and recent history;
- protocol-aware session presentation.

### Terminal

- interactive SSH terminal;
- independent terminal state per active session;
- terminal resize and disconnect;
- non-interactive command execution for controlled AI operations.

### Remote files

- SFTP browsing and navigation;
- upload/download;
- bounded remote-file editing;
- local filesystem validation for transfer targets.

### AI assistance

- OpenAI-compatible cloud providers;
- local OpenAI-compatible provider;
- persistent conversation state;
- active-session infrastructure context;
- diagnostics and health analysis;
- native tool calling through the registered tool set.

### Human-in-the-loop operations

AI-generated infrastructure actions follow the existing execution contract:

1. AI proposes an operation.
2. Command Policy evaluates the operation.
3. Allowed operations execute through the established executor.
4. Operations requiring approval are surfaced to the user.
5. The approved operation resumes the AI workflow.
6. Execution results are bounded before returning to AI/UI.
7. The operation lifecycle is recorded by the centralized audit path.

### Secure credentials

Functional features consume credentials through the existing secure-storage boundary. Product UI and AI context use metadata/presence information rather than secret values.

## Product boundaries

Future features must follow these rules:

1. **UI → service only.** Frontend code does not access storage or transport implementations directly.
2. **Connections use the connection boundary.** A new transport does not add transport-specific state to unrelated models.
3. **AI uses the provider boundary.** Provider-specific HTTP/authentication details stay out of the UI and domain contracts.
4. **Commands use the existing executor.** No feature may create a second path to SSH command execution.
5. **Credentials stay in secure storage.** Do not add secrets to settings, sessions, AI context, logs, or ordinary DTOs.
6. **Audit remains centralized.** User-visible operations that already participate in the audit model must continue to do so.
7. **Security invariants are regression-tested.** New functionality must preserve the Phase 3 security regression suite.

## Explicit non-goals

Phase 5 does not attempt to finish every planned product feature. In particular, the following remain ordinary future feature work:

- richer RDP experience;
- additional credential providers;
- advanced Vault/KeePass workflows;
- additional AI providers;
- macOS and additional Linux packaging;
- advanced infrastructure workflows and automation;
- further UI/UX refinement.

These features should be implemented incrementally without reopening the application architecture unless a concrete correctness or security requirement demands it.

## Definition of done

Phase 5 is complete when:

- the existing terminal, session, SFTP and AI workflows are treated as the stable product surface;
- AI-assisted operations use the existing context, policy, approval, execution and audit pipeline;
- functional additions have a documented place in the product workflow;
- there is no new parallel storage, credential, transport or command-execution path;
- future roadmap items can be implemented as focused product PRs.

## Development rule after Phase 5

> **Build features, not foundations.**
>
> Revisit architecture only when a concrete security, correctness, scalability, or platform requirement cannot be satisfied by the existing boundaries.
