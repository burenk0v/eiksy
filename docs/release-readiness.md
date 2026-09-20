# Eiksy Release Readiness

This checklist is the final engineering gate before creating a release tag. It verifies the product contract, security boundaries, CLI/TUI surface and release artifacts without introducing release-only behavior.

## 1. Source state

- [ ] The release branch is based on the intended `main` commit.
- [ ] The working tree represented by the release candidate contains no temporary implementation files.
- [ ] No deprecated Opsy references remain in source, documentation or release metadata.
- [ ] RDP support is absent from source and documentation.
- [ ] No `.fail` files or related failure-artifact functionality are present.
- [ ] The permanent documentation set contains only current product, architecture, AI, CLI and release references.

## 2. Automated validation

Run the complete backend test suite:

```bash
go test ./...
```

Run the frontend build:

```bash
cd frontend
npm ci
npm run build
```

The pull request must have green:

1. backend and frontend checks;
2. security checks;
3. desktop binary build.

A documentation-only change may intentionally skip the PR test workflow because CI ignores `*.md` changes. Before a release, run the backend and frontend commands explicitly rather than treating a skipped documentation workflow as validation.

## 3. Product acceptance

The release candidate must satisfy [release-acceptance.md](release-acceptance.md):

- [ ] Think → Connect → Operate → Think completes.
- [ ] Infrastructure context is bounded and contains no credentials.
- [ ] Sensitive operations require explicit approval.
- [ ] Approved operations use the existing backend executor.
- [ ] Results are bounded before returning to AI/UI.
- [ ] Executed operations appear in the redacted audit trail.
- [ ] Cancellation and AI operation limits remain bounded.

The acceptance tests named by the release gate must remain green.

## 4. CLI/TUI smoke test

Build or run the CLI and verify:

```bash
eiksy --help
eiksy --version
eiksy tui
eiksy tui --no-alt-screen
eiksy tui --view chat
eiksy tui --view terminal
eiksy tui --view files
eiksy tui --view tools
```

Verify manually:

- [ ] TUI starts in Chat by default.
- [ ] Initial view selection opens the requested tab.
- [ ] Alternate-screen mode can be disabled.
- [ ] Session selection works.
- [ ] Chat input accepts normal printable characters.
- [ ] Terminal input accepts normal printable characters.
- [ ] Files view renders its application-provided state.
- [ ] Command palette opens and closes.
- [ ] Keyboard shortcut overlay opens and closes.
- [ ] Chat session fork works.
- [ ] Security-sensitive AI actions show an approval request.
- [ ] Approval resolution remains inside the existing application boundary.

## 5. Security review

Before tagging the release:

- [ ] No credentials, provider tokens, private keys or certificates are committed.
- [ ] AI conversation and TUI state do not expose decrypted secrets.
- [ ] CLI/TUI does not access secure storage or transports directly.
- [ ] There is one command execution path.
- [ ] Command policy and approval cannot be bypassed from the CLI/TUI.
- [ ] Audit records remain bounded and redacted.
- [ ] The Go vulnerability scan is green.
- [ ] No new credential provider, executor or parallel transport path was added during release preparation.

## 6. Release artifact

- [ ] Release version is decided and follows the repository's versioning convention.
- [ ] Release notes describe user-visible changes only.
- [ ] Release notes do not describe removed or deprecated functionality as available.
- [ ] Linux and Windows artifacts are produced by the release build process.
- [ ] Artifact names and platforms are verified before publication.
- [ ] Release assets do not contain temporary build or test files.
- [ ] The release tag points to the exact validated commit.

## 7. Final decision

The release is ready only when every required checkbox above is satisfied and the release candidate has been validated from the exact commit that will be tagged.

This document is a release-engineering checklist. The product contract, architecture and AI architecture remain the sources of truth for product behavior and security boundaries.
