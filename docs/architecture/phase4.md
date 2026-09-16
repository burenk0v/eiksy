# Phase 4 — Product Foundation

Phase 4 establishes stable product-facing boundaries so future features do not need to redesign the core architecture.

## Scope

- Session profiles remain the user-facing connection configuration and history surface.
- `internal/domain/connections` defines a transport-neutral connection lifecycle and keeps command execution as a separate capability.
- `internal/domain/ai.Provider` defines the runtime boundary for AI backends.
- Settings remain application configuration; secret values continue to live in secure storage.
- Existing audit/event infrastructure remains the single operational audit path.

## Non-goals

This phase does not rewrite existing SSH/SFTP implementations or the current AI HTTP implementation. The new domain contracts are intentionally small and can be adopted incrementally without changing the security boundaries already established in Phases 1–3.

## Rules for future features

1. New transports implement the connection boundary instead of adding transport-specific state to unrelated domain objects.
2. New AI backends implement `ai.Provider` instead of adding provider-specific calls to UI code.
3. Credential material never crosses domain/UI DTO boundaries unless an operation explicitly requires a runtime secret and is already inside the secure execution boundary.
4. Command execution remains behind capability/policy checks and the existing executor/audit pipeline.
5. UI code consumes service APIs and does not access storage or transport implementations directly.

## Acceptance criteria

- New transports have a stable domain contract.
- New AI backends have a stable domain contract.
- Domain contracts contain no secret material.
- Existing command execution remains behind the established policy/capability pipeline.
- The contracts have regression tests.
