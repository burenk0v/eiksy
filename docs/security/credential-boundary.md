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

Vault and KeePass remain application-side credential sources. Their contents must never enter AI infrastructure context or ordinary workspace state. Decrypted values should be resolved directly into connection operations and must not be returned to the UI or AI context.

## Browser authentication

Browser authorization tokens are consumed entirely by the backend callback. The token is written directly to secure storage and is not copied into the Wails-facing authentication-session DTO. The UI receives only authorization status, message, endpoint and session metadata.

The UI is not a security boundary. Authorization, secret storage, execution, and audit decisions belong to the backend.
