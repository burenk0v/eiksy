# Phase 4 — Product Foundation

Phase 4 freezes the product-facing boundaries used by future features.

## Scope

- Session profiles remain the user-facing connection configuration and history surface.
- `domain/connections` defines transport-neutral connection lifecycle and keeps command execution as a separate capability.
- `domain/ai.Provider` defines the runtime boundary for AI backends.
- Settings remain application configuration; secret values continue to live in secure storage.
- Existing audit/event infrastructure remains the single operational audit path.

## Non-goals

This phase does not rewrite existing SSH/SFTP implementations or the current AI HTTP implementation. The new domain contracts are intentionally small and can be adopted incrementally without changing the security boundaries already established in Phases 1–3.

## Acceptance criteria

- New transports have a stable domain contract.
- New AI backends have a stable domain contract.
- Domain contracts contain no secret material.
- Existing command execution remains behind the established policy/capability pipeline.
- The contracts have regression tests.

Future feature work should build on these contracts instead of introducing new cross-layer dependencies.
