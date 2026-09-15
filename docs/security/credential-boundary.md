# Credential boundary

Eiksy treats credentials as application-owned secrets, not AI context.

## Rules

- Session passwords and SSH key passphrases are stored through secure storage and are not persisted in `sessions.json`.
- AI provider tokens are stored through secure storage and are not persisted in the normal AI workspace state.
- The AI infrastructure context contains runtime facts only. Credential material, secret references, and opaque connection options are excluded.
- Command execution is a separate trust boundary. The AI cannot bypass Command Policy by selecting a different execution path.
- Direct access to common credential material is denied by the default shell policy, including `/etc/shadow`, `/etc/gshadow`, SSH private material under `~/.ssh`, `.env*` files, environment variables, and shell history.
- File searches and ordinary file reads remain approval-gated because legitimate diagnostics may require them, but they are treated as potentially secret-bearing operations.
- Audit records must not contain raw credentials, authentication tokens, or secret values.

## Vault and KeePass

Vault and KeePass remain application-side credential sources. Their contents must never be injected into the AI infrastructure context or persisted into ordinary workspace state.

Browsing these stores is a privileged application operation. A future hardening pass should minimize the lifetime of decrypted values and should prefer resolving a credential directly into the connection operation rather than returning the secret to the UI.

## Cloud browser authentication

The browser authorization flow currently uses a short-lived in-memory token during the hand-off from the browser callback to the application. This token must not be written to disk or audit records.

A follow-up hardening change should complete the boundary by removing the token from the Wails-facing authentication-session DTO and consuming the callback token entirely in the backend when saving the provider configuration.

## Security principle

The UI is not a security boundary. The backend owns secret storage, command authorization, execution, and audit decisions. AI prompts are guidance only; authorization is enforced by backend policy.
