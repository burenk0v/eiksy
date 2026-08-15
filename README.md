# opsy

Backend-first cross-platform workstation client built with Go and Wails.

## Current scaffold

This repository now contains the initial application skeleton for:

- protocol registry with SSH, SFTP, and RDP descriptors
- session manager models for saved profiles, launch history, and active tabs
- credential provider contracts for HashiCorp Vault, KeePass, and Windows Password Manager
- AI provider models for local and OpenAI-compatible backends
- backend-owned workspace state rendered by a thin Wails frontend shell

## Development

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
