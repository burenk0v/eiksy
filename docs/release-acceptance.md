# Eiksy Release Acceptance Gate

This is the final product-level verification gate before a release.

## Required workflow

The release must preserve the continuous workflow:

**Think → Connect → Operate → Think**

The acceptance suite must demonstrate:

| Contract | Required evidence |
| --- | --- |
| Think | A user task starts an AI workflow and bounded infrastructure context is available |
| Connect | A configured resource can be connected and its active session is visible |
| Secret isolation | Credentials are excluded from AI context, conversation state and audit output |
| Operate | AI proposes a typed operation without executing it automatically |
| User control | A security-sensitive operation waits for explicit approval |
| Execution | Approval dispatches through the existing backend executor |
| Result | Execution produces a bounded result |
| Audit | Executed operations are represented in the bounded audit trail |
| Operate → Think | A result is returned to the AI and can drive another operation |
| Resilience | AI request cancellation, response-size and turn limits remain bounded |

## Automated evidence

The release gate is covered by:

- `TestThinkConnectOperateAcceptance` — complete product workflow;
- `TestOperateThinkLoopContinuesAfterOperationResult` — repeated operation continuation;
- native-tool approval and command-policy regression tests;
- operation-result bounding tests;
- audit lifecycle and redaction tests;
- AI operation resilience tests.

Run the complete backend suite:

```bash
go test ./...
```

Run the frontend build:

```bash
cd frontend
npm ci
npm run build
```

## CI gate

A release candidate requires green validation for:

1. backend tests;
2. security checks;
3. desktop binary build.

The gate does not introduce a second execution path or a test-only product implementation.

## Security gate

Before release, verify that:

- no credentials, private keys or provider tokens are committed;
- AI context contains only bounded non-secret infrastructure metadata;
- sensitive command execution remains behind the existing policy and approval boundary;
- audit values remain bounded and redacted;
- deprecated RDP support or documentation has not been reintroduced.

## Definition of Done

The product-level DoD remains defined by [product-contract.md](product-contract.md). This document is the release verification gate for that contract; it does not replace the product contract or architecture reference.
