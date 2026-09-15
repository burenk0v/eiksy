# AI Infrastructure Health Analysis

Eiksy diagnostics now returns a deterministic infrastructure health report alongside the raw diagnostic checks.

## Statuses

- `healthy` — no detected threshold violations and all checks completed.
- `degraded` — a diagnostic check failed or a warning threshold was reached.
- `critical` — a critical memory or disk threshold was reached.

## Current rules

### Memory

The analyzer uses the `available` value from `free -h`:

- below 20% of total memory: `warning`
- below 10%: `critical`

### Disk

The analyzer evaluates filesystem usage reported by `df -h`:

- above 90%: `warning`
- above 95%: `critical`

### Failed diagnostics

A non-zero exit code or execution error produces a `warning` finding and changes overall status to `degraded`, unless another finding is already `critical`.

## Security model

The analyzer does not execute commands and does not accept commands from the AI. It consumes the bounded structured results produced by `ssh.diagnostics`. The existing diagnostics path continues to enforce Command Policy before executing its fixed read-only command set.

The health report is therefore an interpretation of collected facts, not an additional execution capability.
