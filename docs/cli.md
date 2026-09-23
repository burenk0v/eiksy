# Eiksy CLI

The Eiksy CLI provides the terminal UI (TUI) as an additional interaction surface for the same Eiksy application core.

The CLI does not introduce a separate session store, credential store, command executor, AI provider implementation, or security boundary.

## Start Eiksy

Running Eiksy without arguments starts the desktop application:

```bash
eiksy
```

To start the terminal UI:

```bash
eiksy tui
```

The TUI uses the same application services and backend security controls as the desktop application.

## Help and version

Show CLI help:

```bash
eiksy --help
```

The following forms are also supported:

```bash
eiksy -h
eiksy help
```

Show the installed version:

```bash
eiksy --version
```

or:

```bash
eiksy -v
eiksy version
```

## TUI options

### Disable alternate-screen mode

By default, the TUI uses the terminal alternate screen. Use `--no-alt-screen` when the terminal should keep the existing screen contents:

```bash
eiksy tui --no-alt-screen
```

This is a presentation option only. It does not change application behavior or security controls.

### Select the initial view

Use `--view` to choose the view shown when the TUI starts:

```bash
eiksy tui --view chat
eiksy tui --view terminal
eiksy tui --view files
eiksy tui --view tools
eiksy tui --view ai
```

The available views are:

| View | Purpose |
| --- | --- |
| `chat` | AI conversation and operation workflow |
| `terminal` | Terminal interaction surface for the active session |
| `files` | File browsing and presentation |
| `tools` | Tool-related interaction surface |
| `ai` | AI interaction surface |

If `--view` is omitted, the TUI starts in `chat`.

Options can be combined:

```bash
eiksy tui --no-alt-screen --view terminal
```

## TUI navigation

The TUI uses a keyboard-first layout inspired by classic terminal file managers and editors. The active view is always visible in the header, and the footer shows the primary actions.

| Key | Action |
| --- | --- |
| `F2` | Sessions |
| `F3` | Terminal |
| `F4` | Files |
| `F5` | Tools |
| `F6` | AI |
| `F10` | Exit |
| `←` / `→` | Previous / next view |
| `Tab` / `Shift+Tab` | Next / previous view |
| `↑` / `↓` | Previous / next session |
| `Enter` | Select session or submit active input |
| `Ctrl+P` | Open command palette |
| `Ctrl+F` | Fork the active chat session |
| `Ctrl+Q` | Quit |
| `?` | Show keyboard shortcuts |
| `Esc` | Close an active overlay |

Printable characters belong to the active input field. Global navigation uses function keys and control combinations so that normal terminal/chat text is not consumed as an application command.

The Terminal view is only interactive when a connected runtime session is bound to it. With no active runtime session, it shows an explicit empty state and does not accept terminal input.

The Files view currently presents application-provided file metadata only; it does not add file-management or editor capabilities to the CLI.
## Sessions

Chat sessions are application-owned and persistent. The CLI displays session metadata and uses application services to select or fork sessions.

The CLI does not maintain a separate session database.

Persistent conversation content remains owned by the application and secure-storage boundary. Presentation state should not be treated as a security boundary.

## AI operations and approvals

AI operations use the existing Eiksy backend path:

```text
AI
 ↓
Application service
 ↓
Policy / capability checks
 ↓
Approval when required
 ↓
Existing executor
 ↓
Infrastructure
 ↓
Bounded result
 ↓
AI + audit
```

The TUI only presents this workflow. It does not execute SSH/SFTP operations directly and does not bypass command policy, approval, credential handling, bounded results, or audit.

When an operation requires approval, the TUI presents the approval request and allows the user to resolve it through the existing application boundary.

## Exit codes

The CLI uses the following exit codes for command-line handling:

| Code | Meaning |
| --- | --- |
| `0` | Command completed successfully |
| `1` | TUI/application execution returned an error |
| `2` | Invalid or unknown command-line input |

For example, an unsupported view returns exit code `2`:

```bash
eiksy tui --view unknown
```

## Architecture

The CLI/TUI is a frontend, not a second Eiksy application:

```text
Eiksy core
    ↓
Application services
    ↓
Domain / security boundaries

Wails UI ─────┐
CLI / TUI ────┴──→ Application services
```

See [the CLI/TUI architecture ADR](adr/0001-cli-tui-architecture.md) for the stable frontend boundary and extension rules.

## Related documentation

- [Architecture](architecture.md)
- [AI architecture](ai.md)
- [Product contract](product-contract.md)
- [Release acceptance](release-acceptance.md)
