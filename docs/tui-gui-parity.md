# TUI / GUI Functional Parity Contract

## Purpose

This document defines the functional contract that the TUI must implement to match the current Eiksy GUI.

The GUI is the reference implementation for functionality during the parity phase. The TUI may use a different navigation model and presentation, but a user must be able to perform the same supported operations.

This is a **parity contract**, not a feature roadmap. It does not add new product capabilities.

## Rules

1. GUI and TUI use the same application/domain services and persisted state.
2. A capability available to the GUI is considered part of the TUI parity target.
3. UI-only presentation details are not part of parity.
4. TUI-specific keyboard navigation is allowed and expected.
5. New functionality must not be introduced into TUI solely because it is convenient to implement.
6. Features removed from the GUI are not parity requirements.
7. Security-sensitive operations must preserve the same validation, approval and secret-isolation boundaries in both interfaces.

## Functional areas

### 1. Application state

The TUI must expose the same operational state available to the GUI:

- session profiles;
- active runtime sessions;
- session history;
- supported protocols;
- application settings;
- AI provider state;
- AI conversation state;
- command-policy state;
- relevant connection/error state.

The TUI must refresh from the shared application state rather than maintaining a separate persistent model.

### 2. Session profiles

The TUI must provide the GUI-equivalent lifecycle for session profiles:

- list existing profiles;
- create a profile;
- edit/update an existing profile;
- delete a profile;
- launch/open a profile as a runtime session;
- reconnect a runtime session;
- disconnect a runtime session;
- close a runtime session.

Profile creation/editing must support the same profile data exposed by the GUI, including:

- name;
- protocol;
- host;
- port;
- username;
- authentication method/options;
- password where supported;
- SSH key passphrase where supported;
- SSH/session options;
- tags/favorite metadata where exposed by the GUI.

Secrets must continue to be stored through secure storage and must not become part of normal TUI state or AI context.

### 3. SSH terminal

For an active SSH session, the TUI must provide the same operational terminal capabilities as the GUI:

- send terminal input;
- receive terminal output;
- resize the terminal;
- execute a command;
- reconnect;
- disconnect;
- close the runtime session;
- handle an unknown SSH host key through the same explicit acceptance flow;
- surface connection and command errors.

The TUI must never accept terminal input as if a usable session existed when no active connected session is selected.

### 4. SFTP

Where the GUI exposes SFTP operations, the TUI must expose the same operations:

- list remote files/directories;
- navigate remote directories;
- read a remote file;
- save/write a remote file;
- upload local files;
- download remote files;
- select the required local file(s)/directory through an appropriate TUI interaction.

The TUI presentation can differ from GUI dialogs, but the resulting operations and validation must be equivalent.

### 5. AI interaction

The TUI must provide the GUI-equivalent AI workflow:

- view AI state/conversation;
- send a chat message;
- associate the request with the active operational session where supported;
- clear/reset the current AI conversation;
- select the configured AI provider;
- configure the supported cloud provider;
- list cloud models where supported;
- start the supported cloud authentication flow;
- observe cloud authentication state;
- configure the local provider;
- download the local model;
- start the local model;
- stop the local model.

AI command execution must preserve the same Eiksy command-policy boundary:

- display pending command requests;
- allow the user to resolve a request;
- preserve the available permission modes;
- update command policy;
- keep command audit information available where exposed by the GUI.

### 6. Settings

The TUI must expose the same user-configurable application settings as the GUI.

The TUI must use the same persisted settings and update path. It must not maintain a TUI-only configuration file.

Settings parity includes all currently supported fields exposed through the GUI, including:

- theme;
- default protocol;
- window/layout-related settings where applicable;
- AI approval/cloud-model settings;
- SSH port-forwarding rules;
- SSH configuration import state;
- supported credential-provider configuration;
- secure-storage status and lifecycle where exposed by the GUI.

Sensitive values must remain protected by the existing secure-storage boundary.

### 7. Secure storage

The TUI must provide equivalent access to the GUI secure-storage lifecycle where the GUI exposes it:

- inspect secure-storage status;
- initialize/configure the master password;
- lock secure storage;
- surface locked/unavailable states;
- retry operations that require unlocked secure storage.

The TUI must never display secret values merely because it needs to report their configured/present state.

### 8. SSH configuration import

The TUI must provide the same user-visible SSH configuration import capability as the GUI.

Automatic import behavior performed during application startup remains an application-level behavior and must not be duplicated by the TUI.

### 9. Errors and transient states

Every parity operation must have an explicit TUI representation for:

- loading/in-progress;
- success where user feedback is required;
- validation failure;
- connection failure;
- operation failure;
- unavailable/locked secure storage;
- empty state;
- disconnected state.

Errors must not terminate the TUI process.

## Explicit non-goals

The following are not part of this parity contract unless they are first exposed by the GUI:

- new TUI-only product capabilities;
- a separate TUI configuration format;
- separate session storage;
- a separate AI implementation;
- compatibility flags or legacy TUI invocation modes;
- CLI options for functionality that belongs to the interactive UI.

## Implementation boundary

The intended architecture is:

```
                    Shared application/domain layer
                              /          \
                             /            \
                           GUI            TUI
```

GUI and TUI should call the same session, settings, AI, storage and security services wherever the operation already exists in the application layer.

If a GUI capability cannot currently be reached through the shared application layer, the next implementation step is to expose that existing capability through the shared layer rather than reimplementing it inside TUI.

## Definition of done for parity contract

- [ ] Every GUI capability listed above has a corresponding TUI interaction.
- [ ] Session creation is available from TUI.
- [ ] Session editing and deletion are available from TUI.
- [ ] Runtime session lifecycle is available from TUI.
- [ ] SSH terminal input/output is available from TUI.
- [ ] SFTP operations exposed by GUI are available from TUI.
- [ ] AI interaction and provider management are available from TUI.
- [ ] Command approval/policy controls are available from TUI.
- [ ] Settings are read and written through the shared application state.
- [ ] Secure-storage lifecycle is preserved.
- [ ] No TUI-only persistence or duplicate business logic is introduced.
- [ ] Tests cover the shared application operations used by both interfaces.
- [ ] Linux and Windows TUI builds are verified.

## Current reference

This contract is based on the GUI/application surface present on `main` at the time this document was introduced. It should be updated when an intentional GUI capability is added or removed.

The next implementation stages should use this document as the acceptance checklist rather than inventing a separate TUI feature set.
