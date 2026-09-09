<p align="center">
  <img src="frontend/src/assets/images/logo.png" alt="opsy logo" width="180" />
</p><h1 align="center">opsy</h1><p align="center">
  <strong>Ops. Secure. You.</strong><br>
  Secure remote operations workstation with AI at your side.
</p><p align="center">
  <a href="https://github.com/burenk0v/opsy/releases/latest">
    <img src="https://img.shields.io/github/v/release/burenk0v/opsy?label=latest%20release" alt="Latest Release" />
  </a>
  <a href="https://github.com/burenk0v/opsy/actions">
    <img src="https://img.shields.io/github/actions/workflow/status/burenk0v/opsy/build-binaries.yml?label=build" alt="Build" />
  </a>
  <a href="https://github.com/burenk0v/opsy/blob/main/LICENSE">
    <img src="https://img.shields.io/github/license/burenk0v/opsy" alt="License" />
  </a>
</p><p align="center">
  <a href="https://github.com/burenk0v/opsy/releases/latest">Download</a> ·
  <a href="https://github.com/burenk0v/opsy/issues">Issues</a> ·
  <a href="https://github.com/burenk0v/opsy/blob/main/SECURITY.md">Security</a>
</p>---

What is opsy?

opsy is a cross-platform remote operations workstation built with Go and Wails.

It brings remote access, file management, credentials, and AI-assisted operations into a single desktop application.

Instead of switching between terminal clients, SFTP tools, RDP applications, password managers, and AI assistants, opsy aims to provide one secure workspace for working with infrastructure.

«Ops. Secure. You.»

---

Features

Remote operations

- SSH terminal sessions
- SFTP file browsing and transfers
- Remote file editing
- RDP support
- Saved connection profiles
- Session history and active tabs
- SSH config import
- ProxyJump support
- SSH agent support
- Local SSH tunnels
- Encrypted SSH key passphrases

Credentials

opsy provides an abstraction layer for external credential providers:

- HashiCorp Vault
- KeePass
- Windows Password Manager

Sensitive local data is stored in an encrypted SQLite database.

AI

opsy supports OpenAI-compatible AI providers with configurable:

- API endpoint
- authentication token
- model
- provider configuration

This makes it possible to connect both cloud-based and self-hosted AI services.

The goal is not to build another chat application.

The goal is to make AI a natural part of everyday infrastructure operations.

---

Security

Security is a core part of the architecture rather than an additional feature.

Sensitive local data is protected using:

- OS keychain-backed master password flow
- Argon2id key derivation
- XChaCha20-Poly1305 encryption
- encrypted SQLite secret storage
- external secret providers
- least-privilege oriented architecture

The application is designed so that credentials and secrets do not need to be stored in plaintext configuration files.

Security principles

opsy follows several basic principles:

- Secrets should not be stored in plaintext.
- Credentials should be separated from application configuration.
- External secret stores should be supported whenever possible.
- The AI layer should not automatically receive unrestricted access to infrastructure.
- Operations should remain under user control.

Security issues should be reported privately according to the project's security policy:

"SECURITY.md"

---

Architecture

opsy follows a backend-first architecture.

┌──────────────────────────────────────────┐
│                Wails UI                  │
│          Desktop / Web Frontend          │
└────────────────────┬─────────────────────┘
                     │
                     ▼
┌──────────────────────────────────────────┐
│              Go Backend                  │
│                                          │
│  Session Manager                         │
│  Protocol Registry                       │
│  SSH / SFTP / RDP                        │
│  Credential Providers                    │
│  AI Providers                            │
│  Secret Storage                          │
└───────────────┬───────────────┬──────────┘
                │               │
                ▼               ▼
       ┌────────────────┐  ┌───────────────┐
       │ Remote Systems │  │ Secret Stores │
       │ SSH / RDP /    │  │ Vault /       │
       │ SFTP           │  │ KeePass / OS  │
       └────────────────┘  └───────────────┘

The frontend is intentionally kept relatively thin.

Core application logic belongs to the Go backend, making the system easier to test, maintain, and extend.

---

AI-assisted operations

AI is becoming part of infrastructure engineering, but giving an AI unrestricted access to production infrastructure creates obvious security risks.

opsy is designed around a different approach:

User
  │
  ▼
opsy
  │
  ├── Remote session
  │
  ├── Credentials
  │
  ├── Infrastructure context
  │
  └── AI assistant
          │
          ▼
      Suggested action
          │
          ▼
         User

The user remains the final decision maker.

The long-term goal is to provide useful AI assistance without turning the workstation into an uncontrolled autonomous agent.

---

Download

Download the latest available release from GitHub:

Latest release

"https://github.com/burenk0v/opsy/releases/latest"

Currently supported release targets include:

- Windows x64
- Linux x64

More platforms may be added as the project evolves.

---

Installation

Windows

1. Download the latest Windows binary.
2. Extract the application.
3. Start "opsy.exe".

Linux

1. Download the latest Linux binary.
2. Make it executable:

chmod +x opsy-linux-amd64

3. Start the application:

./opsy-linux-amd64

---

Development

Requirements

- Go 1.25+
- Node.js 20+
- Wails CLI 2.14.0
- Platform-specific Wails dependencies

Clone the repository:

git clone https://github.com/burenk0v/opsy.git
cd opsy

Install frontend dependencies:

cd frontend
npm ci
cd ..

Run the application in development mode:

wails dev

Build the application:

wails build

---

Testing

The project is developed with a backend-first approach where core functionality can be tested independently from the desktop UI.

Before submitting changes, make sure the project builds successfully and relevant tests pass.

For local development:

go test ./...

---

CI/CD

GitHub Actions automatically builds desktop binaries for supported platforms.

The release workflow:

1. Builds the application.
2. Produces platform-specific binaries.
3. Uploads build artifacts.
4. Creates a GitHub Release when a semantic version tag is pushed.
5. Attaches the binaries to the release.

To create a release:

git tag vX.Y.Z
git push origin vX.Y.Z

The README intentionally does not contain a hard-coded release number.

The "latest" release is always referenced dynamically.

---

Configuration and secrets

Do not commit credentials, API keys, private keys, tokens, or other sensitive information to the repository.

Local sensitive data should be stored through the application's encrypted secret storage or an external credential provider.

Environment files containing secrets should remain outside version control.

---

Project status

opsy is an actively evolving open-source project.

The current focus is on building a reliable foundation for:

- remote infrastructure access
- secure credential management
- AI-assisted operations
- cross-platform desktop workflows
- extensible protocol support

Some components are still evolving and may change between releases.

---

Roadmap

Potential areas of development include:

- additional remote protocols
- improved RDP integration
- richer terminal functionality
- advanced Vault integration
- additional credential providers
- AI-powered infrastructure diagnostics
- AI-assisted command generation
- controlled AI execution workflows
- approval and audit mechanisms
- session recording
- improved cross-platform support
- plugins and extensions

The roadmap is intentionally flexible as the project develops.

---

Why Go + Wails?

Go

Go provides:

- excellent networking capabilities
- strong concurrency primitives
- a small runtime footprint
- cross-platform support
- easy distribution as a native binary
- a mature ecosystem for infrastructure tooling

Wails

Wails makes it possible to combine:

- a native Go backend
- a modern web-based UI
- desktop application capabilities

This combination allows opsy to keep infrastructure logic in Go while maintaining a flexible user interface.

---

Development philosophy

opsy is developed using a pragmatic, AI-assisted approach.

AI tools are used as development accelerators, while architecture, security decisions, testing, and final implementation remain under human control.

The project follows a vibe-coding style:

«Rapid iteration, pragmatic decisions, continuous refinement.»

The goal is not to pretend that AI can replace engineering.

The goal is to use AI to increase development speed while keeping engineering responsibility where it belongs.

---

Contributing

Contributions, bug reports, ideas, and security improvements are welcome.

Before opening a pull request:

- keep changes focused;
- avoid committing secrets;
- add or update tests where appropriate;
- keep the architecture consistent with the existing project;
- document significant behavior changes.

For security vulnerabilities, please follow "SECURITY.md" instead of opening a public issue.

---

Support

If you find opsy useful and want to support its development, you can find donation information in "SUPPORT.md".

---

License

opsy is released under the Apache License 2.0.

See "LICENSE" for details.

---

<p align="center">
  <strong>Ops. Secure. You.</strong>
</p>