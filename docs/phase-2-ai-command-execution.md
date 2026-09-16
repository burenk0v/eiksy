# Phase 2 — AI Command Execution

Phase 2 establishes the final backend contract for AI infrastructure command execution.

## Invariants

1. AI can only request registered tools.
2. `ssh.exec` is evaluated by Command Policy before execution.
3. Approval is resolved by the backend and resumes the same tool conversation.
4. Execution uses the existing SSH connection through a dedicated non-interactive exec channel.
5. Commands are single, non-composed commands and have a bounded input size.
6. Execution results have bounded stdout, stderr and error fields before entering AI context or UI.
7. Every policy decision and execution outcome is recorded by the audit pipeline.
8. No UI or future CLI path may bypass the backend execution boundary.

## Completion criterion

New AI functionality must use the existing tool registry, Command Policy, approval lifecycle, executor and audit path. A new feature must not introduce a second command-execution path.
