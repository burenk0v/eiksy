# Credential boundary follow-up

The current hardening change establishes backend denial for direct access to common secret material.

Remaining work for a subsequent change:

1. Remove cloud browser auth tokens from the Wails-facing DTO.
2. Let the backend consume and persist the browser callback token without returning it to the frontend.
3. Minimize decrypted Vault/KeePass value lifetime and avoid returning secret values through UI APIs where possible.
4. Add explicit regression tests proving Vault/KeePass values never enter AI context, shell state, workspace events, or audit records.
