# Eiksy AI Architecture

Eiksy treats AI as an infrastructure operations assistant, not as an autonomous security authority.

> **AI may understand infrastructure and propose operations, but only Eiksy's backend policy may authorize execution.**

This document is the permanent reference for Eiksy's AI architecture and security model. It describes the current AI capabilities, boundaries, execution lifecycle, context model, diagnostics and remediation rules. It does not describe development phases or temporary implementation milestones.

## 1. AI architecture

```text
┌────────────────────────────────────────────────────────────┐
│                         Eiksy UI                            │
└──────────────────────────┬─────────────────────────────────┘
                           │
                           ▼
┌────────────────────────────────────────────────────────────┐
│                    AI Application Service                  │
│                                                            │
│  conversation state / context / tool dispatch / approval  │
└───────────────┬────────────────────────────┬───────────────┘
                │                            │
                ▼                            ▼
      ┌──────────────────┐        ┌────────────────────────┐
      │  AI Provider     │        │ Infrastructure Context │
      │                  │        │                        │
      │ cloud / local    │        │ runtime facts only     │
      └──────────────────┘        └────────────────────────┘
                │
                ▼
        AI response / tool call
                │
                ▼
      ┌──────────────────────┐
      │ Command / Tool Policy│
      │ capability + approval│
      └──────────┬───────────┘
                 │
                 ▼
           Existing Executor
                 │
                 ▼
          Infrastructure
                 │
                 ▼
          Bounded result
                 │
          ┌──────┴──────┐
          ▼             ▼
         AI            Audit
```

The model has no direct reference to SSH, filesystem or credential implementations. Native tool calls are dispatched by the application service and evaluated before execution.

## 2. AI provider boundary

Eiksy uses an application-level AI provider contract so that cloud and local models participate in the same workflow.

The provider boundary hides:

- HTTP/API protocol details;
- authentication implementation;
- provider-specific request/response formats;
- endpoint-specific configuration.

The application should communicate with AI through provider-neutral contracts.

### Provider types

The architecture supports:

- OpenAI-compatible cloud providers;
- local/self-hosted OpenAI-compatible providers;
- future providers implementing the same application contract.

Provider tokens are credentials. They belong in secure storage and must never enter ordinary AI workspace state or infrastructure context.

## 3. Conversation state

Conversation state represents the model interaction required to continue an AI workflow.

It may contain:

- messages;
- tool calls;
- tool results;
- bounded infrastructure context;
- pending approval continuation data.

It must not become a general-purpose secret store.

Credentials, decrypted Vault/KeePass values and provider authentication material remain outside conversation state.

When a command requires approval, Eiksy preserves the pending native tool call and serialized conversation context so the same AI workflow can continue after the user's decision.

The pending request is cleared before continuation to prevent repeated execution of the same approved operation.

## 4. Infrastructure context

AI receives a small, backend-built infrastructure context for the currently selected session.

The context can contain operational facts such as:

- active Eiksy session ID and display name;
- protocol and connection status;
- host, port and username;
- session description;
- profile group and tags;
- current remote directory when it can be obtained safely;
- other explicitly approved, bounded infrastructure facts.

The context is constructed by the backend, not by the frontend and not by the model.

### Excluded data

The following must not enter AI infrastructure context:

- passwords;
- SSH key passphrases;
- provider tokens;
- decrypted Vault/KeePass values;
- secret references whose purpose is to expose secret material;
- arbitrary environment dumps;
- opaque credential-bearing connection options.

A failure to obtain one optional fact must not make the entire context unusable.

### Context authority

Infrastructure context is informational. It does not grant execution authority.

The model knowing the target host does not mean the model is authorized to run a command on that host.

## 5. Native tools

AI interacts with infrastructure through explicit native tools rather than direct access to backend implementations.

The tool set should follow the principle of **small, typed and policy-controlled capabilities**.

A tool should expose only the operation it needs to perform. Generic backdoors such as arbitrary access to an SSH client, credential store or filesystem should not be exposed to the model.

Current infrastructure-oriented tools include the controlled command execution path and fixed read-only diagnostics.

## 6. AI command execution

AI command execution is a controlled infrastructure operation, not unrestricted shell access.

```text
AI tool call
    ↓
Command Policy
    ├── deny → audit + tool error
    ├── allow → execute → bounded result → audit
    └── ask → approval
                  ├── deny → audit
                  └── approve → execute once → bounded result → audit
```

The built-in `ssh.exec` capability:

- executes one non-interactive command in an existing SSH session;
- uses the existing SSH connection through a separate exec channel;
- is disabled unless the corresponding policy tool is enabled;
- rejects shell composition and expansion syntax before execution;
- supports explicit allow, approval and deny rules;
- defaults unmatched commands to approval when command rules are configured.

### Shell restrictions

The command policy deliberately rejects shell composition constructs including:

- `;`
- `&&`
- `||`
- `|`
- redirection (`>` / `<`)
- command substitution and shell expansion;
- backslashes;
- embedded newlines.

The policy prefers explicit approval over silently interpreting a compound command.

### Approval modes

A policy request can be resolved as:

- **now** — execute this request once without persisting permission;
- **session** — allow this exact command for the current session;
- **always** — allow this exact command for future matching requests;
- **deny** — reject the request.

Persisted permissions remain command rules evaluated with tool/session specificity before execution.

## 7. Result handling

Execution results are bounded before they enter AI context or UI payloads.

Structured results can include:

- status;
- target session;
- command;
- exit code;
- duration;
- bounded stdout/stderr information;
- bounded execution error information.

The purpose of bounding is to prevent infrastructure commands from turning into unbounded AI context or UI payloads and to reduce accidental exposure of large sensitive datasets.

Errors and results are subject to the same redaction requirements as commands and audit events.

## 8. AI security boundary

The model is not trusted with application authority.

The security boundary is the backend combination of:

```text
Context rules
     +
Tool definitions
     +
Command Policy
     +
Capability checks
     +
Human approval
     +
Controlled executor
     +
Result bounding
     +
Central audit
```

The UI is an approval surface, not a security boundary.

A future CLI, automation interface or alternate frontend must use the same backend controls.

## 9. Infrastructure diagnostics

Diagnostics provide deterministic, read-only infrastructure observations for AI-assisted analysis.

The diagnostic operation executes a fixed read-only command set through the existing policy-controlled execution path. The analyzer does not execute commands and does not accept commands from the model.

Its input is the bounded structured output produced by the diagnostics tool.

### Health status

The deterministic health analyzer produces:

- `healthy` — no detected threshold violations and all checks completed;
- `degraded` — a diagnostic check failed or a warning threshold was reached;
- `critical` — a critical memory or disk threshold was reached.

### Current thresholds

**Memory** is evaluated from available memory reported by `free -h`:

- below 20% of total memory → warning;
- below 10% → critical.

**Disk** is evaluated from filesystem usage reported by `df -h`:

- above 90% → warning;
- above 95% → critical.

A non-zero diagnostic exit code or execution error creates a warning finding and changes overall status to `degraded`, unless another finding is already critical.

Malformed or unrecognized output is not converted into a guessed finding.

The health report is an interpretation of collected facts, not an additional execution capability.

## 10. AI remediation

Remediation follows a deliberately conservative lifecycle:

**Detect → Analyze → Propose → Approve → Execute → Verify**

### Detect

Use fixed, read-only diagnostics to collect infrastructure facts.

Diagnostic output is treated as untrusted data, not as instructions.

### Analyze

Use the deterministic health report and bounded underlying diagnostic results.

The model must not invent facts that are absent from the infrastructure context or diagnostic result.

### Propose

The AI explains the finding and intended remediation and states the exact command when a command is required.

The proposed change should be the smallest reasonable operation that addresses the observed finding.

### Approve

Remediation commands use the existing `ssh.exec` path.

`Command Policy` remains the only application execution gate. Approval uses the normal pending-request flow.

### Execute

Only the established SSH command execution path performs the change.

Existing command restrictions, policy rules, approval semantics, audit and result bounding remain in force.

### Verify

After a successful change, diagnostics are run again.

The new health state and relevant check are compared with the pre-remediation state. A zero exit code alone is not sufficient evidence that remediation succeeded.

## 11. Explicit remediation limits

There is intentionally no unrestricted `ssh.remediate` or generic remediation executor.

Current AI remediation does not introduce:

- automatic destructive repair;
- package installation;
- privilege escalation;
- credential access;
- arbitrary shell composition;
- a second command-execution path.

Future remediation capabilities require explicit validation, policy coverage, approval semantics, audit events and post-change verification before exposure to the model.

## 12. AI audit model

AI operations participate in the centralized audit lifecycle.

For command operations, important states include:

- `approval_required`;
- `policy_denied`;
- `executed`;
- `execution_failed`.

Approval resolution is audited so the operation lifecycle distinguishes the AI proposal, the user's decision and the resulting execution.

Audit records must not contain:

- passwords;
- provider tokens;
- private keys;
- decrypted credential values;
- raw authentication material.

Command, result and error fields must remain bounded and redacted.

## 13. AI security invariants

1. **AI never receives credentials.**
2. **AI context is informational, not authoritative.**
3. **AI cannot call transport implementations directly.**
4. **Every infrastructure tool has an explicit backend capability boundary.**
5. **Every command passes through Command Policy before execution.**
6. **Approval cannot be bypassed by the executor or UI.**
7. **There is only one command execution path.**
8. **Diagnostic analysis cannot execute commands.**
9. **Diagnostic output is treated as untrusted data.**
10. **Command output and errors are bounded before downstream AI processing.**
11. **Audit remains centralized and redacted.**
12. **Remediation is verified after execution.**
13. **New AI tools must not create parallel credential, transport or execution paths.**

## 14. Advantages

### Controlled autonomy

AI can perform useful infrastructure work without being granted unrestricted infrastructure authority.

### Provider independence

Cloud and local models participate in the same application workflow, reducing provider-specific coupling.

### Security by construction

Tool calls, policy, approval, execution, result bounding and audit form a small number of inspectable control points.

### Deterministic diagnostics

Health analysis is separated from command execution, making the interpretation layer predictable and testable.

### Safe remediation model

The Detect → Analyze → Propose → Approve → Execute → Verify lifecycle prevents a successful command exit code from being mistaken for verified infrastructure recovery.

### Context minimization

The model receives only the infrastructure information needed for the task rather than the complete session configuration or credential state.

## 15. Trade-offs and limitations

### Less automation

Approval and policy can make some operations slower than giving an agent unrestricted shell access.

**Trade-off:** reduced convenience in exchange for explicit operator control over sensitive actions.

### Less context

The model cannot freely inspect every file, environment variable or credential.

**Trade-off:** some investigations require targeted tools, explicit approval or additional user input.

### More tooling code

Typed native tools and policy boundaries require more implementation than exposing a generic shell.

**Trade-off:** the resulting attack surface and authorization model are easier to reason about.

### Conservative command syntax

Compound shell commands are rejected.

**Trade-off:** multi-step tasks may require several explicit tool calls instead of one shell expression.

### Bounded results

Large output may be truncated.

**Trade-off:** complex investigations may need targeted commands, pagination or dedicated diagnostic tools.

## 16. Extension rules

When adding AI functionality:

1. add the smallest explicit native capability needed;
2. keep provider-specific behavior behind `ai.Provider`;
3. keep credentials outside AI state and context;
4. define the capability's policy and approval behavior before exposing it to the model;
5. reuse existing executors whenever the operation already has an execution path;
6. bound all data entering AI context;
7. treat remote output as untrusted data;
8. audit security-relevant operations through the centralized path;
9. add regression tests for new security invariants;
10. do not create generic escape hatches for convenience.

A new AI capability may require a new explicit service or executor when the operation is genuinely different. It must still preserve the same security principles.

## 17. Architecture change policy

The AI architecture is considered stable.

Revisit it only when a concrete requirement cannot be satisfied safely and correctly within the current boundaries, for example:

- a demonstrated security flaw;
- a correctness problem in the current execution model;
- a required AI capability that cannot fit the existing provider/tool contracts;
- a scalability or context-management limitation that cannot be solved locally;
- a fundamental change in the product's AI operating model.

A new AI feature alone is not sufficient justification for creating another architecture.

> **Build AI capabilities, not AI escape hatches.**
