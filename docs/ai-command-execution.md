# AI Command Execution

Eiksy treats AI command execution as a controlled infrastructure operation, not as unrestricted shell access.

## Execution flow

```text
AI
 |
 | tool call: ssh.exec
 v
Command Policy
 |
 +-- deny ----------> audit + tool error
 |
 +-- allow ---------> execute -> result -> audit
 |
 +-- ask ------------> approval request
                         |
                         +-- deny ------> audit
                         |
                         +-- now -------> execute once
                         |
                         +-- session ---> persist exact command/session rule
                         |
                         +-- always ----> persist exact command rule
                                      |
                                      v
                                  execute
                                      |
                                      v
                                   result
                                      |
                                      v
                                    audit
```

## Security boundary

The model never receives a direct reference to the SSH implementation. Native tool calls are dispatched by the application service and are evaluated by `CommandPolicy` before execution.

The built-in `ssh.exec` tool:

- executes one non-interactive command in an existing SSH session;
- uses the existing SSH connection but a separate exec channel;
- is disabled unless the corresponding policy tool is enabled;
- rejects shell composition and expansion syntax before execution;
- supports explicit allow, approval and deny rules;
- defaults unmatched commands to approval when command rules are configured.

The following shell constructs are rejected by policy rather than delegated to the remote shell:

- `;`
- `&&`
- `||`
- `|`
- redirection (`>` / `<`)
- command substitution (backticks and `$...` expansion)
- backslashes
- embedded newlines

This is intentionally conservative. Eiksy should prefer an explicit approval over silently interpreting a compound command.

## Approval modes

An approval request can be resolved as:

- **now** — execute this request once without persisting a permission rule;
- **session** — allow this exact command for the current session;
- **always** — allow this exact command for future matching requests;
- **deny** — reject the request.

Persisted permissions are represented as command rules and are evaluated with tool/session specificity before execution.

## Audit

Command operations produce bounded audit events covering the policy decision and execution lifecycle. Audit data must not contain credential material or raw authentication secrets.

Important states include:

- `approval_required`
- `policy_denied`
- `executed`
- `execution_failed`

Approval resolution is audited as part of the same lifecycle so an operator can distinguish a proposed operation, the user's decision and the resulting execution.

## Tool-call continuation

When a command requires approval, Eiksy persists the pending native tool call and the serialized conversation context. Resolving the request resumes the same tool conversation with the command result instead of starting a new unrelated AI request.

A pending request is cleared before continuation. This prevents the same approval request from being executed repeatedly after a successful resolution.

## Result handling

Command execution returns structured information to the model, including:

- status;
- target session;
- command;
- exit code;
- duration;
- bounded stdout/stderr result information;
- execution error information when applicable.

Command output is bounded to prevent an infrastructure command from turning into an unbounded AI context or UI payload.

## Design rule

The core invariant is:

> **AI may propose an infrastructure operation, but only Eiksy's policy gate may authorize execution.**

The UI is an approval surface, not the security boundary. Even if a future UI or CLI is added, command execution must continue through the same backend policy and audit path.
