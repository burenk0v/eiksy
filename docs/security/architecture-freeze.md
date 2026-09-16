# Eiksy Security Architecture Freeze

This document defines the security boundaries that all future functionality must use. It is intentionally prescriptive: new features should extend the existing boundaries instead of introducing parallel execution, storage, credential, or audit paths.

## 1. Trust boundaries

```text
Wails UI / future CLI
        |
        v
Application service
        |
        +--> sessions / settings
        |
        +--> secure storage
        |
        +--> AI context
        |
        +--> command policy / approval
        |
        +--> protocol executors
        |       +--> SSH
        |       +--> SFTP
        |       +--> filesystem
        |
        +--> audit
```

The UI is never a security boundary. Authorization, secret access, execution and audit decisions are backend responsibilities.

## 2. Credentials

Credential material is application-owned secret state.

- Session passwords and SSH key passphrases live in secure storage.
- AI provider tokens live in secure storage.
- Session/profile DTOs expose only metadata and presence flags.
- AI context must contain runtime facts only; it must not contain credential material, secret values, secret references, or opaque credential-bearing connection options.
- Vault/KeePass are credential sources, not AI data sources.
- Decrypted credentials must be resolved as close as possible to the operation that consumes them and must not be returned through UI-facing diagnostic/context APIs.

## 3. AI boundary

AI may propose operations but does not receive direct executor references.

Every infrastructure operation requested by AI must pass through the application policy boundary before reaching an executor.

```text
AI request
   -> command/tool policy
   -> deny | approval | allow
   -> executor
   -> bounded result
   -> audit
   -> AI
```

There must be no alternate AI-to-SSH, AI-to-filesystem, AI-to-SFTP, or AI-to-process execution path.

## 4. Command execution

The command executor is the single execution boundary for AI commands.

- Policy is evaluated before execution.
- Approval is enforced by the backend, not by UI state.
- Persisted permissions are explicit command rules.
- Compound shell syntax is rejected by the command policy rather than delegated to the remote shell.
- Results are bounded before entering AI context or UI payloads.
- Execution and approval lifecycle events are audited.

## 5. Filesystem and SFTP

Filesystem and SFTP access are capabilities, not ambient privileges.

- Path validation happens in the backend.
- Credential paths are capability-scoped.
- SFTP connections require trusted host verification.
- File content must not bypass the execution/audit security model when exposed to AI.
- New file operations must reuse the existing filesystem/SFTP boundary rather than accessing the OS directly from UI code.

## 6. Audit

Audit is a security boundary, not merely a log sink.

- Audit fields are sanitized at the final recording boundary.
- Command, result and error data are bounded.
- Credential/token/secret material must never be persisted in audit events.
- New execution paths must emit the same lifecycle audit events.

## 7. Serialization and Wails

Wails DTOs are treated as externally visible application interfaces.

A DTO must not expose secret material merely because the backend needs it internally. Sensitive state should be kept in backend-only structures and consumed there.

When adding a new DTO, explicitly classify every field as one of:

- public metadata;
- runtime state;
- capability/policy state;
- secret material (backend-only).

Secret material must never be part of the Wails-facing category.

## 8. Storage lifecycle

Persistent state is divided into:

- normal application state: settings, session metadata, workspace state and non-secret configuration;
- secure state: passwords, passphrases, provider tokens and other secrets.

Normal state must remain usable without embedding secret values. Locking secure storage must invalidate access to decrypted secrets and clear transient key material according to the secure-storage contract.

## 9. Rules for future development

A feature is architecturally acceptable when it can be implemented by composing existing boundaries.

Do not introduce:

- a second command execution path;
- a second secret store;
- direct secret reads from UI/Wails code;
- AI access to secure storage;
- unbounded command output in AI context;
- direct filesystem/SSH/SFTP access from AI code;
- audit bypasses;
- security decisions implemented only in the frontend.

If a new feature appears to require one of these, the architecture must be revisited explicitly rather than silently extending the existing model.

## 10. Phase 1 exit criteria

Phase 1 is complete when:

1. all execution paths use the backend capability/policy boundary;
2. credentials remain outside normal state and AI context;
3. Wails/UI DTOs contain no secret material;
4. filesystem/SSH/SFTP boundaries are backend-enforced;
5. audit is centralized and sanitized;
6. security invariants are represented by automated tests where practical;
7. future features can be implemented without creating a new security boundary.

After this point, architectural changes should be driven by a concrete security defect or a genuinely new trust boundary, not by feature-level refactoring preference.
