# Phase 3 — Security Regression Invariants

Phase 3 turns the security boundaries established in the previous hardening work into executable regression checks.

## Invariants

- Secrets must not cross the shell/UI serialization boundary.
- Credential material must not enter the AI infrastructure context.
- Command audit records must redact command, result, and error secret material.
- Disabled AI tools must not be accepted by command policy resolution.

The regression suite is intentionally focused on security invariants rather than implementation details. Future feature work should preserve these invariants without creating alternate execution or data paths.
