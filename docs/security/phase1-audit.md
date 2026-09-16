# Phase 1 architecture audit — 2026-09-16

Baseline: `main` at `1dbe44b7b9c5ca18c5ed61a9a5ea4fd38b6bdad4`.

## Audit scope

- sessions
- settings
- secure storage
- credentials
- AI context
- command policy/execution
- SSH/SFTP/filesystem
- audit
- Wails/UI exposure

## Findings and implemented boundaries

### Sessions / settings

Session DTOs expose connection metadata and secret-presence flags rather than stored password/key material. Secret values are resolved through the backend secure-storage path.

### Secure storage

Secure storage is the authoritative location for passwords, SSH key passphrases and AI provider tokens. The master key lifecycle is explicitly locked and transient key material is cleared on close.

### AI context

The infrastructure context is backend-built and intentionally excludes credentials, secret references and opaque credential-bearing options. AI does not receive direct SSH implementation references.

### Command execution

AI command execution is policy-gated. The backend distinguishes deny, approval and allow paths and records the lifecycle in the centralized command audit trail. The executor is not exposed directly to the model.

### SSH / SFTP / filesystem

SSH and SFTP access are backend operations. SFTP host verification is explicit and filesystem/credential path validation is performed in backend capability code. Future file operations must continue to use these boundaries.

### Audit

Command audit values pass through a final redaction and bounded-text sanitizer, including command, result and error fields.

### Wails/UI

The UI is treated as an untrusted presentation layer. Backend policy, credential access, execution and audit remain authoritative.

Browser authorization was hardened as part of Phase 1: the callback token is written directly to secure storage and the completed Wails-facing `CloudProviderAuthSession` contains no token value. A regression test verifies both properties.

## Phase 1 decision

No new parallel execution, credential, storage or audit architecture should be introduced. The existing boundaries are now explicit and the remaining browser-auth credential exposure has been removed.

Phase 1 is ready for the regression suite and PR review. Phase 2 can focus on formalizing the AI command-execution contract rather than reopening the storage/session architecture.
