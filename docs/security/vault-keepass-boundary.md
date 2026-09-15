# Vault and KeePass credential boundary

Eiksy keeps Vault and KeePass as optional application-side credential sources, but does not expose a secret browser.

## User-facing model

- Users configure a Vault/KeePass provider and an explicit secret path.
- Eiksy can validate that path against the configured provider without returning the secret value.
- Vault/KeePass tree browsing and refresh controls are removed from the UI and Wails bindings.
- Secret values are not copied into AI context, infrastructure diagnostics, workspace state, or audit events.

## Validation

`ValidateCredentialPath(provider, path)` is the narrow validation boundary. It:

1. validates the provider (`vault` or `keepass`);
2. normalizes the path and rejects empty paths, NUL bytes, and traversal outside the provider root;
3. queries only the parent path needed to verify the requested leaf;
4. rejects directory paths;
5. returns success/error only and never returns the secret value.

The validation operation is deliberately not a general-purpose secret browsing API.

## Security boundary

The intended flow is:

`Vault/KeePass -> backend -> connection operation`

not:

`Vault/KeePass -> UI/AI -> secret value -> connection`

Provider credentials remain in secure storage. The provider path itself is configuration and may be shown to the user, but secret material must not be emitted through Wails DTOs, AI prompts, diagnostics, or audit logs.

## Browser/authentication scope

The Vault/KeePass secret-tree browser is removed rather than hidden. There are no user-facing tree, directory-navigation, refresh, or secret-list bindings.

Vault/KeePass authentication remains application-side where required for explicit path validation. Browser-based secret selection is not required.
