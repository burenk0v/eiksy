# Security hardening

Security-sensitive Eiksy behavior is enforced in the backend.

- [Credential boundary](credential-boundary.md)
- Command Policy is the authorization boundary for AI command execution.
- Secure storage owns persisted credentials and authentication secrets.
- AI infrastructure context is intentionally non-secret.

Security changes should include regression tests for both the positive workflow and the forbidden data flow.
