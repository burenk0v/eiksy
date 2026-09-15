# AI Infrastructure Context

Eiksy exposes a small, backend-built infrastructure context for AI-assisted operations.

## Purpose

The context gives the model enough information to reason about the currently selected SSH session without exposing credentials or opaque connection configuration.

The current vertical slice contains:

- active Eiksy session ID and display name;
- protocol and connection status;
- host, port and username;
- session description;
- profile group and tags;
- current remote directory when it can be obtained safely.

## Security boundary

The context is constructed by `Service.GetAIInfrastructureContext`, not by the frontend and not by the model. The following are deliberately excluded:

- password;
- SSH key passphrase;
- secret references;
- provider tokens;
- arbitrary session options.

A failure to obtain the current directory does not fail the whole context. This keeps diagnostics useful even when a session is partially available.

## Execution relationship

Infrastructure context is informational. It does not grant execution authority. Any command proposed by the model still goes through the existing `ssh.exec` → Command Policy → approval/execution → audit pipeline.

This separation is intentional: context helps the model understand the target, while Command Policy remains the authorization boundary.

## Next increment

The next increment can add bounded recent terminal output and command history, controlled by `ContextPolicy`, without putting raw terminal state or credentials into persistent AI state.
