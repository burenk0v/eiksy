# Eiksy AI Architecture

Eiksy treats AI as an infrastructure operations assistant, not as an autonomous security authority.

> **AI may understand infrastructure and propose operations, but only Eiksy's backend policy may authorize execution.**

This document is the permanent reference for Eiksy's AI architecture and security model. It describes the current AI capabilities, boundaries, execution lifecycle, context model, diagnostics and remediation rules. It does not describe development phases or temporary implementation milestones.

## 1. AI architecture

```text
Eiksy UI
    ↓
AI Application Service
    ├── AI Provider
    ├── Infrastructure Context
    └── Native Tool Dispatch
             ↓
       Command / Tool Policy
             ↓
          Approval
             ↓
       Existing Executor
             ↓
       Infrastructure
             ↓
        Bounded result
             ↓
        AI + Audit
```

The model has no direct reference to SSH, filesystem or credential implementations. Native tool calls are dispatched by the application service and evaluated before execution.

## 2. AI provider boundary

Eiksy keeps AI provider selection and request handling inside the application service. Cloud and local providers expose the same OpenAI-compatible request workflow without an unused provider interface.

Supported provider types include OpenAI-compatible cloud providers and local/self-hosted OpenAI-compatible providers. Provider tokens are credentials and must remain in secure storage.

## 3. Conversation state

Conversation state represents the model interaction required to continue an AI workflow. It may contain messages, tool calls, tool results, bounded infrastructure context and pending approval continuation data.

It must not become a general-purpose secret store. Credentials, decrypted Vault/KeePass values and provider authentication material remain outside conversation state.

When a command requires approval, Eiksy preserves the pending native tool call and serialized conversation context so the same workflow can continue after the user's decision. The pending request is cleared before continuation to prevent repeated execution of the same approved operation. Starting a new chat clears the conversation, pending approval requests and pending native tool continuation state together, so an operation from the previous AI session cannot be resumed accidentally.

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

### Context sensitivity

Operational context is not automatically public just because it is not a credential. In particular, remote paths can contain sensitive project, customer or environment names. Paths and similar metadata must therefore remain bounded and must not be treated as credentials or expanded into arbitrary filesystem discovery.

### Context authority

Infrastructure context is informational. It does not grant execution authority.

The model knowing the target host does not mean the model is authorized to run a command on that host.

## 5. Native tools

AI interacts with infrastructure through explicit native tools rather than direct access to backend implementations.

The tool set follows the principle of **small, typed and policy-controlled capabilities**. A tool should expose only the operation it needs to perform. Generic backdoors such as arbitrary access to an SSH client, credential store or filesystem must not be exposed to the model.

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

Shell composition constructs including `;`, `&&`, `||`, pipes, redirection, command substitution, shell expansion, backslashes and embedded newlines are rejected. The policy prefers explicit approval over silently interpreting a compound command.

Approval modes are `now`, `session`, `always` and `deny`. Persisted permissions remain command rules evaluated with tool/session specificity before execution.

## 7. Result handling

Execution results are bounded before they enter AI context or UI payloads.

Structured results can include status, target session, command, exit code, duration and bounded stdout/stderr and error information.

Bounding prevents commands from turning into unbounded AI context or UI payloads and reduces accidental exposure of large sensitive datasets. Errors and results are subject to the same redaction requirements as commands and audit events.

## 8. AI security boundary

The model is not trusted with application authority.

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

The UI is an approval surface, not a security boundary. A future CLI, automation interface or alternate frontend must use the same backend controls.

## 9. Infrastructure diagnostics

Diagnostics provide deterministic, read-only infrastructure observations for AI-assisted analysis.

The diagnostic operation executes a fixed read-only command set through the existing policy-controlled execution path. The analyzer does not execute commands and does not accept commands from the model.

Health analysis uses the existing deterministic thresholds for memory and disk. A non-zero diagnostic exit code or execution error creates a warning finding and changes overall status to `degraded`, unless another finding is already critical. Malformed or unrecognized output is not converted into a guessed finding.

The health report is an interpretation of collected facts, not an additional execution capability.

## 10. AI remediation

Remediation follows:

**Detect → Analyze → Propose → Approve → Execute → Verify**

Diagnostics collect facts. Analysis uses only bounded results. The model proposes the smallest reasonable change and states the exact command when one is required. Approval uses the normal `ssh.exec` path. Verification runs diagnostics again and compares the resulting health state; a zero exit code alone is not sufficient evidence that remediation succeeded.

## 11. Explicit remediation limits

There is intentionally no unrestricted `ssh.remediate` or generic remediation executor.

Current AI remediation does not introduce automatic destructive repair, package installation, privilege escalation, credential access, arbitrary shell composition or a second command-execution path.

Future remediation capabilities require explicit validation, policy coverage, approval semantics, audit events and post-change verification before exposure to the model.

## 12. AI audit model

AI operations participate in the centralized audit lifecycle. Command operations distinguish states such as `approval_required`, `policy_denied`, `executed` and `execution_failed`.

The current command audit is a **bounded local operational trail**, not a compliance storage or SIEM backend. Disk-backed application state persists the bounded trail across restarts; command/error/result values pass through centralized redaction.

Audit records must not contain passwords, provider tokens, private keys, decrypted credential values or raw authentication material.

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

## 14. Extension rules

When adding AI functionality:

1. add the smallest explicit native capability needed;
2. keep provider-specific behavior behind `ai.Provider`;
3. keep credentials outside AI state and context;
4. define policy and approval behavior before exposing a capability to the model;
5. reuse existing executors whenever the operation already has an execution path;
6. bound all data entering AI context;
7. treat remote output as untrusted data;
8. audit security-relevant operations through the centralized path;
9. add regression tests for new security invariants;
10. do not create generic escape hatches for convenience.

A new AI capability may require a new explicit service or executor when the operation is genuinely different. It must still preserve the same security principles.

## 15. Architecture change policy

The AI architecture is considered stable.

Revisit it only when a concrete requirement cannot be satisfied safely and correctly within the current boundaries, for example a demonstrated security flaw, a correctness problem in the current execution model, a required AI capability that cannot fit the existing provider/tool contracts, a scalability limitation that cannot be solved locally or a fundamental change in the product's AI operating model.

A new AI feature alone is not sufficient justification for creating another architecture.

> **Build AI capabilities, not AI escape hatches.**
