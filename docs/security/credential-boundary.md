# Credential boundary

Eiksy treats credentials as application-owned secrets, not AI context.

- Session passwords and SSH key passphrases are stored through secure storage and are not persisted in `sessions.json`.
- AI provider tokens are stored through secure storage and are not persisted in normal AI workspace state.
- AI infrastructure context contains runtime facts only; credential material, secret references, and opaque connection options are excluded.
- AI command execution remains behind the backend Command Policy and audit boundary.
- The default policy denies direct access to common credential material such as `/etc/shadow`, `/etc/gshadow`, SSH private material under `~/.ssh`, `.env*`, environment variables, and shell history.
- Ordinary file reads and searches remain approval-gated because they may expose sensitive data.
- Audit records must not contain raw credentials, authentication tokens, or secret values.

## Vault and KeePass

Vault and KeePass remain application-side credential sources. Their contents must never enter AI infrastructure context or ordinary workspace state. A later hardening change should minimize decrypted-value lifetime and prefer resolving credentials directly into connection operations instead of returning secret values to the UI.

## Browser authentication

The browser authorization flow uses a short-lived in-memory token during the callback hand-off. A follow-up hardening change should remove that token from the Wails-facing authentication-session DTO and consume it entirely in the backend when saving provider configuration.

The UI is not a security boundary. Authorization, secret storage, execution, and audit decisions belong to the backend.
