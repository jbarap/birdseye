# agents-install Specification

## Purpose
TBD - created by archiving change agents-install-workflows. Update Purpose after archive.
## Requirements
### Requirement: Type-scoped install command

The system SHALL provide `be agents install <type>` and `be agents uninstall <type>`,
where `<type>` names an agent integration (e.g. `claude`), so integrations for additional
agent types can be added without reshaping the command. The legacy `be hook claude
install` and `be hook claude uninstall` commands SHALL remain as hidden, deprecated
aliases that route to the hooks tier, so existing instructions keep working.

#### Scenario: Install is scoped by agent type

- **WHEN** the user runs `be agents install claude`
- **THEN** the command targets the Claude Code integration's install behavior

#### Scenario: Deprecated hook alias still works

- **WHEN** the user runs `be hook claude install`
- **THEN** the system performs the hooks-tier install (the alias is accepted though hidden
  from help)

### Requirement: Two independent opt-in tiers

The install command SHALL expose two independent tiers selected by `--hooks` and
`--workflows`: **hooks** (agent status detection) and **workflows** (birdseye-authored
skills/commands). Passing a flag SHALL install that tier; passing both SHALL install both.
The tiers SHALL be independent — installing or removing one SHALL NOT require or affect the
other. No tier SHALL ever be installed implicitly.

#### Scenario: Install only the hooks tier

- **WHEN** the user runs `be agents install claude --hooks`
- **THEN** only the status-detection hooks are installed and no workflow artifacts are
  written

#### Scenario: Install only the workflows tier

- **WHEN** the user runs `be agents install claude --workflows`
- **THEN** only the workflow artifacts are written and no hooks are installed

#### Scenario: Install both tiers

- **WHEN** the user runs `be agents install claude --hooks --workflows`
- **THEN** both the hooks and the workflow artifacts are installed

### Requirement: Interactive selection when no tier flag is given

The system SHALL, when neither `--hooks` nor `--workflows` is given in an interactive
terminal, present a checklist of the tiers with a one-line explanation of each, install
exactly the tiers the user selects, and treat selecting nothing as a no-op. In a
non-interactive context (no TTY), the no-flag form SHALL fail with a message telling the
caller to pass `--hooks` and/or `--workflows`, rather than prompt or install anything
implicitly.

#### Scenario: Checklist shown with no flags

- **WHEN** the user runs `be agents install claude` with no tier flag in a terminal
- **THEN** the system shows a checklist explaining the hooks and workflows tiers and
  installs only what the user selects

#### Scenario: Selecting nothing installs nothing

- **WHEN** the user is shown the checklist and selects no tier
- **THEN** the system writes nothing and exits without changes

#### Scenario: No-flag form refuses in a non-TTY

- **WHEN** `be agents install claude` runs with no tier flag and no interactive terminal
- **THEN** the system exits non-zero asking for `--hooks` and/or `--workflows` and installs
  nothing

### Requirement: Hooks tier preserves safe install behavior

The hooks tier SHALL install and uninstall the agent's status hooks with the existing
safety guarantees: it SHALL preserve all unrelated configuration and hooks, support a
user-supplied settings path, record a backup before changing the settings file, operate
idempotently, refuse to modify malformed settings, and on uninstall remove only the entries
birdseye added.

#### Scenario: Hooks install preserves existing config

- **WHEN** the user installs the hooks tier into a settings file containing other keys and
  unrelated hooks
- **THEN** the tool adds its hook entries while leaving all other configuration intact and
  records a backup

#### Scenario: Hooks install is idempotent

- **WHEN** the user installs the hooks tier when it is already present and unchanged
- **THEN** the tool reports no change and does not duplicate entries

#### Scenario: Hooks uninstall removes only its own entries

- **WHEN** the user uninstalls the hooks tier
- **THEN** only birdseye's hook entries are removed and all other configuration remains

### Requirement: Workflows tier ships removable artifacts

The workflows tier SHALL write birdseye-authored skill/command artifacts — shipped
embedded in the binary — into the agent's configuration in a birdseye-namespaced
location, so they can be removed without touching the user's own skills. Install SHALL be
idempotent (rewriting only changed artifacts) and SHALL back up or refuse to clobber a
user-modified artifact rather than silently overwrite it. The artifacts SHALL be removable
with no effect on the binary, the `be agents` verbs, or the hooks tier. The shipped set
SHALL include at least an orchestrator workflow and QA and PR workflows that compose the
`be agents` verbs.

#### Scenario: Workflows install writes namespaced artifacts

- **WHEN** the user installs the workflows tier
- **THEN** the embedded skill/command artifacts are written into a birdseye-namespaced
  location in the agent's config

#### Scenario: Workflows install is idempotent

- **WHEN** the user installs the workflows tier when the artifacts are already present and
  unchanged
- **THEN** the tool reports no change and does not rewrite unchanged artifacts

#### Scenario: Workflows uninstall removes only birdseye artifacts

- **WHEN** the user uninstalls the workflows tier
- **THEN** only birdseye's artifacts are removed and the user's own skills are left intact

#### Scenario: Removing workflows breaks nothing core

- **WHEN** the workflow artifacts are absent (never installed or uninstalled)
- **THEN** the binary, the `be agents` verbs, and the hooks tier all continue to function

### Requirement: Symmetric tier-scoped uninstall

`be agents uninstall <type>` SHALL accept the same `--hooks` / `--workflows` selection as
install (with a checklist of currently-installed tiers when no flag is given in a terminal)
and SHALL remove only birdseye's entries and artifacts for the chosen tiers.

#### Scenario: Uninstall a single tier

- **WHEN** the user runs `be agents uninstall claude --workflows`
- **THEN** only the workflow artifacts are removed and the hooks tier is left installed

#### Scenario: Uninstall checklist with no flag

- **WHEN** the user runs `be agents uninstall claude` with no flag in a terminal
- **THEN** the system shows a checklist of the currently-installed tiers and removes only
  those the user selects

