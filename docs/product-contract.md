# Eiksy Product Contract

## Think. Connect. Operate.

This document defines the product-level Definition of Done for Eiksy.

It is a permanent product contract, not a development-phase description. Future implementation work should be evaluated against this contract.

## Core principle

Eiksy is complete at the product level when a user can move through a continuous operational workflow:

**Think → Connect → Operate → Think**

The three capabilities are distinct, but they must work together as one workflow.

---

## THINK

Think is the reasoning and planning stage.

Eiksy must allow the user to:

- describe a task in natural language;
- work with an AI session;
- maintain conversation history;
- provide explicit context to the AI;
- understand what context is available to the AI;
- use results from previous operations as input for further reasoning;
- distinguish AI suggestions from actions that are actually executed.

### Think DoD

Think is done when a user can start with a human description of a task and develop it into a concrete operational plan without leaving Eiksy.

AI must not become the security boundary. Authorization and execution control belong to Eiksy.

---

## CONNECT

Connect is the stage where Eiksy makes the required context and resources available to the workflow.

Resources may include systems, remote hosts, AI providers, secret stores and other supported integrations.

Eiksy must provide:

- explicit resource configuration;
- explicit resource capabilities;
- connection lifecycle management;
- isolated credentials;
- least-privilege access;
- explicit boundaries around what the AI and user can access;
- secure secret handling;
- the ability to disconnect or revoke access.

A resource existing in Eiksy does not imply that the AI can use it.

### Connect DoD

Connect is done when a user can securely connect the resources required for a task and make them available to the workflow without creating uncontrolled access.

---

## OPERATE

Operate is the execution stage.

Eiksy must allow the user to:

- select a target resource;
- understand the intended action;
- approve security-sensitive actions;
- execute an action through the appropriate connector;
- observe execution status;
- cancel or time out long-running operations;
- receive a normalized result;
- use the result in subsequent reasoning.

The execution path must be controlled by Eiksy rather than by the AI model itself.

### Operate DoD

Operate is done when a user can safely execute an approved action against a connected resource, observe its result and continue the workflow using that result.

---

## USER CONTROL

The core execution boundary is:

**AI suggests → User reviews → User approves → Eiksy executes**

AI may understand infrastructure and propose operations.

AI must not independently bypass Eiksy authorization, capabilities or execution policy.

---

## SECURITY PRINCIPLES

The Think / Connect / Operate workflow must preserve these boundaries:

- least privilege;
- explicit access;
- secret isolation;
- no plaintext secret persistence where secure storage is available;
- no accidental secret exposure through AI history;
- no accidental secret exposure through logs, audit records or errors;
- auditable security-relevant operations;
- explicit user control over security-sensitive execution.

Security is part of the product contract, not a later optional feature.

---

## END-TO-END DEFINITION OF DONE

The product-level DoD is satisfied when the following workflow works as one continuous loop:

```
THINK
  User describes a task
       ↓
  AI understands the task
       ↓
CONNECT
  Required resources are explicitly connected
       ↓
OPERATE
  AI proposes an operation
       ↓
  User reviews and approves
       ↓
  Eiksy executes the operation
       ↓
  Result is returned
       ↓
THINK
  Result becomes available for further reasoning
       ↓
  Next operation can be planned
```

The loop must support repeated iterations:

**Think → Connect → Operate → Think → Operate → ...**

---

## Acceptance criteria

The product contract is considered implemented when:

- [ ] Think workflow is available.
- [ ] Explicit AI context is available.
- [ ] Connect has explicit resource boundaries.
- [ ] Resource lifecycle is controlled.
- [ ] Secrets are isolated from normal application data and AI history.
- [ ] Operate has a controlled execution path.
- [ ] Security-sensitive operations have explicit user approval.
- [ ] Operations expose status and results.
- [ ] Operations can be cancelled or timed out where applicable.
- [ ] Security-relevant operations are auditable.
- [ ] Operation results can return to the AI context.
- [ ] The complete Think → Connect → Operate → Think workflow is covered by an end-to-end acceptance test.
- [ ] Documentation describes the final product model rather than temporary development stages.

## Out of scope

This contract does not prescribe:

- a particular AI provider;
- a particular connector implementation;
- a particular UI layout;
- a particular storage implementation;
- a particular deployment model.

Those are implementation choices as long as they preserve the product contract and its security boundaries.
