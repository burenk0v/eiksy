<p align="center">
  <img src="https://raw.githubusercontent.com/burenk0v/opsy/main/frontend/src/assets/images/logo.png" alt="opsy logo" width="180" />
</p>

<p align="center">
  <a href="https://github.com/burenk0v/opsy/releases/latest">
    <img src="https://img.shields.io/github/v/release/burenk0v/opsy?display_name=tag&label=latest%20release" alt="Latest Release" />
  </a>
</p>

# opsy

Backend-first cross-platform workstation client built with Go and Wails.

## Current scaffold

This repository now contains the initial application skeleton for:

- protocol registry with SSH, SFTP, and RDP descriptors
- session manager models for saved profiles, launch history, and active tabs
- SSH config import into session profiles, including ProxyJump, SSH agent, and local tunnel options
- SSH terminal tabs with SFTP browsing and in-app remote file editing
- credential provider contracts for HashiCorp Vault, KeePass, and Windows Password Manager
- AI provider models for local and OpenAI-compatible backends
- local Qwen3 8B model download and llama.cpp launch flow alongside configurable cloud endpoint/token setup
- backend-owned workspace state rendered by a thin Wails frontend shell

## Development

### Prerequisites

- Go 1.25+
- Node.js 20+
- Wails CLI 2.14.0

Install the Wails CLI:

```bash
go install github.com/wailsapp/wails/v2/cmd/wails@v2.14.0
```

### Local workflows

Install frontend dependencies once before running frontend build commands directly:

```bash
cd frontend && npm ci
```

Run the frontend-backed desktop app in development mode:

```bash
wails dev
```

Run backend tests:

```bash
go test ./...
```

Build the frontend assets only:

```bash
cd frontend && npm run build
```

## Build release binaries

The compiled binaries are written to `build/bin/`.

### Linux

Install the required system packages and build the binary:

```bash
sudo apt-get update
sudo apt-get install -y --no-install-recommends build-essential libgtk-3-dev libwebkit2gtk-4.1-dev
wails build -clean -platform linux/amd64 -tags webkit2_41 -o opsy-linux-amd64
```

### Windows

Build the Windows binary:

```bash
wails build -clean -platform windows/amd64 -o opsy-windows-amd64.exe
```

## CI artifacts

GitHub Actions workflow `.github/workflows/build-binaries.yml` builds Windows and Linux binaries and uploads them as workflow artifacts on every push, pull request, and manual run.

When you push a Git tag such as `v0.1.0`, the same workflow automatically creates a GitHub Release for that tag and attaches the built Linux and Windows binaries as release assets.
