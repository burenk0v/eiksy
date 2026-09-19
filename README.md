<p align="center">
  <img src="frontend/src/assets/images/logo.png" alt="Eiksy AI operations platform logo" width="180">
</p>

<h1 align="center">Eiksy</h1>

<p align="center">
  <strong>Think. Connect. Operate.</strong><br>
  AI-powered remote operations workstation for infrastructure
</p>

<p align="center">
  <a href="https://github.com/burenk0v/eiksy/releases/latest">Latest release</a> ·
  <a href="https://github.com/burenk0v/eiksy/issues">Issues</a> ·
  <a href="https://github.com/burenk0v/eiksy/security">Security</a>
</p>

---

## What is Eiksy?

**Eiksy is an AI-powered remote operations workstation for infrastructure engineers, DevOps engineers and system administrators.**

It combines AI assistance, terminal access, SSH, SFTP, remote file management and secure credential storage in one cross-platform desktop application.

The key design principle is simple:

> **AI may assist with infrastructure operations, but the backend remains the security boundary.**

## Current capabilities

### AI

- OpenAI-compatible cloud providers.
- Local/self-hosted OpenAI-compatible AI.
- Local Qwen3 4B (Q4_K_M) workflow.
- Backend-built infrastructure context without credentials.
- Typed native tools.
- Controlled `ssh.exec` command execution.
- Command Policy with allow / approval / deny semantics.
- Human approval for sensitive operations.
- Bounded command results.
- Read-only infrastructure diagnostics.
- Conservative AI remediation flow: **Detect → Analyze → Propose → Approve → Execute → Verify**.
- Redacted, bounded command audit trail.

### SSH

- Interactive SSH terminal sessions.
- SSH configuration import.
- `ProxyJump` support.
- SSH agent integration.
- Local SSH tunnels and forwarding rules.
- Saved connection profiles.
- Session history.
- Multiple active sessions.

### SFTP

- Remote browsing and directory navigation.
- File reading and editing.
- Upload and download operations.
- SFTP alongside SSH sessions.

### Credentials

Eiksy keeps credential material separate from ordinary application state and supports credential-provider integrations including Vault, KeePass and local encrypted storage.

Sensitive local data is protected with OS keychain-backed master-password flow, Argon2id key derivation and XChaCha20-Poly1305 encryption.

## Security model

The AI execution path is intentionally narrow:

```text
AI tool call
    ↓
Command Policy
    ↓
Capability / approval
    ↓
Existing executor
    ↓
Infrastructure
    ↓
Bounded result
    ↓
Audit
```

AI does not receive passwords, private keys, API tokens or decrypted credential values. The UI is an interaction surface, not the authorization boundary.

See [`docs/architecture.md`](docs/architecture.md) for application boundaries and [`docs/ai.md`](docs/ai.md) for the AI security model.

## Architecture

Eiksy uses a Go + Wails desktop architecture:

```text
Wails UI
   ↓
Application Services
   ├── Sessions / workspace
   ├── AI
   ├── Security / policy
   └── File / connection operations
          ↓
     Domain contracts
          ↓
       Executors
       ├── SSH / SFTP
       └── Filesystem

Secure Storage and Audit are cross-cutting boundaries.
```

The architecture is considered stable. New features should use the existing boundaries instead of introducing parallel execution, storage or credential paths.

The product contract is **Think → Connect → Operate → Think**. See [`docs/product-contract.md`](docs/product-contract.md) for the product-level Definition of Done and [`docs/architecture.md`](docs/architecture.md) for the implementation boundaries.

## Download

Download the latest release from GitHub:

https://github.com/burenk0v/eiksy/releases/latest

Currently available platforms include Windows x64 and Linux x64.

## Development

### Requirements

- Go 1.25+
- Node.js 20+
- Wails CLI 2.14.0
- Platform-specific Wails dependencies

### Setup

```bash
git clone https://github.com/burenk0v/eiksy.git
cd eiksy
cd frontend
npm ci
cd ..
```

### Development mode

```bash
wails dev
```

### Backend tests

```bash
go test ./...
```

### Frontend build

```bash
cd frontend
npm run build
```

CI runs both backend tests and the frontend build.

## Building

Build output is placed in `build/bin/`.

### Linux

```bash
wails build \
  -clean \
  -platform linux/amd64 \
  -tags webkit2_41 \
  -o eiksy-linux-amd64
```

### Windows

```bash
wails build \
  -clean \
  -platform windows/amd64 \
  -o eiksy-windows-amd64.exe
```

## Project status

Eiksy is an actively developed open-source project. The current focus is feature development on top of the established security and application architecture.

### Roadmap

Planned areas include:

- [ ] Expanded AI-assisted operations
- [ ] Advanced Vault integration
- [ ] Additional credential providers
- [ ] Improved connection import/export
- [ ] More Linux distributions
- [ ] macOS support
- [ ] Automated security testing
- [ ] Improved UI/UX

The roadmap intentionally contains future work only; implemented AI command execution, diagnostics, approval and audit capabilities are documented under **Current capabilities** above.

## Configuration and secrets

Never commit sensitive information to the repository.

Do not commit passwords, API tokens, Vault tokens, SSH private keys or certificates.

Use Eiksy's encrypted storage or an external credential provider for sensitive data.

## Contributing

Before opening a pull request:

- keep changes focused;
- avoid committing secrets;
- add or update tests where appropriate;
- follow the existing architecture;
- document significant behavior changes.

For security vulnerabilities, follow [`SECURITY.md`](SECURITY.md) instead of opening a public issue.

## License

Eiksy is licensed under **Apache-2.0**.

See [`LICENSE`](LICENSE) for the full license text.

<p align="center">
  <strong>Eiksy — Think. Connect. Operate.</strong>
</p>
