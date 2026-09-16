# Product Foundation

Phase 4 establishes stable product-facing boundaries so future features do not need to redesign the core architecture.

## Boundaries

### Sessions

Session profiles own connection metadata and references to credentials. Secret material remains in secure storage and is not part of the connection domain model.

### Connections

`internal/domain/connections` defines the common transport boundary for SSH, SFTP, RDP and local connections. Transport implementations remain responsible for protocol-specific behavior.

Command execution is a separate capability (`CommandExecutor`) and must not be inferred from the ability to connect.

### AI providers

`internal/domain/ai.Provider` is the runtime boundary for AI backends. Provider implementations own endpoint and credential handling; callers operate on `ChatRequest`/`ChatResponse` and do not receive provider secrets.

### Settings

Application settings remain the persisted configuration surface. Secrets such as Vault, KeePass and provider tokens are represented by presence flags at the UI boundary and resolved through secure storage.

### Audit

Operational and AI command activity continues to use the existing audit/event model. New features should emit events through the existing service boundary rather than creating an independent logging path.

## Rules for future features

1. New transports implement the connection boundary instead of adding transport-specific state to unrelated domain objects.
2. New AI backends implement `ai.Provider` instead of adding provider-specific calls to UI code.
3. Credential material never crosses domain/UI DTO boundaries unless an operation explicitly requires a runtime secret and is already inside the secure execution boundary.
4. Command execution remains behind policy/capability checks and the existing executor/audit pipeline.
5. UI code consumes service APIs and does not access storage or transport implementations directly.
