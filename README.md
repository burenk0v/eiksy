<p align="center">
  <img src="frontend/src/assets/images/logo.png" alt="Eiksy AI operations platform logo" width="180">
</p>

<h1 align="center">Eiksy</h1>

<p align="center">
  <strong>Think. Connect. Operate.</strong><br>
  AI-powered remote operations workstation for infrastructure
</p>

<p align="center">
  <a href="https://github.com/burenk0v/eiksy/releases/latest">
    <img src="https://img.shields.io/github/v/release/burenk0v/eiksy?label=latest%20release" alt="Latest Release">
  </a>
  <a href="https://github.com/burenk0v/eiksy/actions">
    <img src="https://img.shields.io/github/actions/workflow/status/burenk0v/eiksy/build-binaries.yml?label=build" alt="Build">
  </a>
  <a href="https://github.com/burenk0v/eiksy/blob/main/LICENSE">
    <img src="https://img.shields.io/github/license/burenk0v/eiksy" alt="License">
  </a>
</p>

<p align="center">
  <a href="https://github.com/burenk0v/eiksy/releases/latest">Download</a> ·
  <a href="https://github.com/burenk0v/eiksy/issues">Issues</a> ·
  <a href="https://github.com/burenk0v/eiksy/security">Security</a>
</p>

---

## What is Eiksy?

**Eiksy is an AI-powered remote operations workstation for infrastructure engineers, DevOps engineers and system administrators.**

It combines **AI assistance, terminal access, SSH, SFTP, RDP, remote file management and secure credential storage** in a single cross-platform desktop application.

Instead of switching between an SSH client, terminal emulator, SFTP client, RDP application, password manager and AI assistant, Eiksy provides one workspace for **secure infrastructure operations**.

Eiksy is designed for people who work with remote servers and infrastructure every day and want AI to become a useful part of their workflow — without giving an AI agent uncontrolled access to production systems.

### In short

**Eiksy = AI + Terminal + SSH/SFTP + RDP + Credentials + Infrastructure Operations**

### Documentation

- [Architecture](docs/architecture.md) — application architecture, boundaries, security invariants and extension rules.
- [AI](docs/ai.md) — AI architecture, context, tools, command execution, diagnostics, remediation and AI security model.

---

## Why Eiksy?

Modern infrastructure workflows often require several separate tools:

```text
Terminal
   +
SSH client
   +
SFTP client
   +
RDP client
   +
Password manager
   +
AI assistant
   =
Too many tools
```

Eiksy brings these workflows together:

```text
                 ┌─────────────────────┐
                 │       Eiksy         │
                 │                     │
                 │  AI Assistant       │
                 │  SSH / SFTP         │
                 │  RDP                │
                 │  File Management    │
                 │  Credentials        │
                 │  Remote Operations  │
                 └─────────────────────┘
                           │
                           ▼
                     Infrastructure
```

The goal is not to build another chat application.

The goal is to make **AI a natural part of everyday infrastructure operations**.

---

## Key features

### AI-assisted infrastructure operations

Eiksy supports OpenAI-compatible AI providers and can connect AI assistance directly to the infrastructure workflow.

Supported configuration includes:

- custom API endpoints;
- authentication tokens;
- model selection;
- provider-specific settings;
- local Qwen3 4B (Q4_K_M) model download and launch;
- custom local model URLs.

Both **cloud AI** and **self-hosted/local AI** workflows are supported.

The long-term direction is an AI assistant that can understand the current infrastructure context and help the operator investigate, diagnose and execute tasks — while keeping the human in control.

### SSH

Eiksy provides a full SSH workflow:

- SSH terminal sessions;
- import of existing SSH configuration;
- `ProxyJump` support;
- SSH agent integration;
- local SSH tunnels;
- encrypted SSH key passphrases;
- saved connection profiles;
- session history;
- multiple active sessions.

Eiksy can therefore be used as a **modern SSH client and terminal workstation** for remote infrastructure.

### SFTP

Remote file operations are available alongside SSH sessions:

- remote file browsing;
- directory navigation;
- file transfers;
- remote file editing;
- SFTP sessions alongside SSH.

This eliminates the need to switch between a terminal and a separate SFTP application for common administration tasks.

### RDP

RDP support is part of the Eiksy remote-session architecture and is actively being developed.

The long-term goal is to provide SSH, SFTP and RDP workflows from the same operations workstation.

---

## Secure credential management

Infrastructure tools handle sensitive information.

Eiksy therefore provides a common credential-provider abstraction for different storage backends:

- HashiCorp Vault;
- KeePass;
- Windows Password Manager;
- local encrypted storage.

Sensitive local data is protected using:

- OS keychain-backed master-password flow;
