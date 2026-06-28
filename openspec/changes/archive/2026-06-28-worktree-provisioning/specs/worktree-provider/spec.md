## ADDED Requirements

### Requirement: Carry git-ignored local files into a new worktree

On creating a worktree, the system SHALL copy the local files a project marks in a
`.worktreeinclude` file at the repository root - the cross-tool convention that uses
`.gitignore` pattern syntax - from the repository's **primary worktree** into the newly
created worktree. A file SHALL be copied only when it both matches an include pattern **and**
is git-ignored, so tracked files are never duplicated; this guardrail SHALL be enforced via
git itself (e.g. `git ls-files --others --ignored` constrained to the include patterns) rather
than a hand-rolled matcher. The relative path of each copied file SHALL be preserved under the
new worktree. The copy SHALL run as part of worktree **creation** (the `be worktree add` /
spawn path), and SHALL NOT run when an existing worktree is re-opened. The copy SHALL be
best-effort: an absent `.worktreeinclude`, or one whose patterns match nothing, is a no-op,
and a copy error (e.g. a permission failure on one file) SHALL be reported as a non-fatal
warning rather than aborting worktree creation, since the worktree is already usable.

#### Scenario: Git-ignored matched files are copied from the primary worktree

- **WHEN** a worktree is created in a repository whose primary worktree holds a git-ignored
  `.env` listed in `.worktreeinclude`
- **THEN** `.env` is copied into the new worktree at the same relative path

#### Scenario: Tracked files are never copied

- **WHEN** `.worktreeinclude` lists a pattern that also matches a tracked file
- **THEN** the tracked file is not copied, because only files that are both matched and
  git-ignored are eligible

#### Scenario: Absent or unmatched include is a no-op

- **WHEN** the repository has no `.worktreeinclude`, or its patterns match no git-ignored file
- **THEN** worktree creation proceeds with nothing copied and no error

#### Scenario: A copy failure does not abort creation

- **WHEN** one listed file cannot be copied (for example a permission error)
- **THEN** the system reports a non-fatal warning and the worktree creation still succeeds

#### Scenario: Re-opening an existing worktree does not re-copy

- **WHEN** an existing worktree (a windowless slot) is re-opened rather than created
- **THEN** the copy step does not run, because the files were carried over at creation time

### Requirement: Run a configured setup command on worktree creation

The system SHALL run a project-defined **setup command** when a worktree is created, so a new
worktree can install dependencies, generate files, or otherwise prepare git-untracked state.
The command SHALL be the `[worktree] setup` value of the **effective configuration** for the
worktree's repository - the user configuration overlaid with the repository's
`.birdseye/config.toml` per the `be-cli` capability - so a project's value overrides the user
default and an unset value at both levels means no setup runs. The command SHALL run **after**
the `.worktreeinclude` copy step, with its working directory set to the new worktree and the
environment variables `BIRDSEYE_WORKTREE` (the new worktree's absolute path), `BIRDSEYE_REPO`
(the primary worktree's absolute path - the copy source), and `BIRDSEYE_BRANCH` (the branch
checked out) exported. Setup SHALL run only on worktree **creation**, never when an existing
worktree is re-opened. For the agentless `be worktree add` path the command SHALL run inline,
streaming its output to the terminal, and a non-zero exit SHALL fail the command (leaving the
created worktree on disk so the user can fix and retry); `be worktree add` SHALL accept a
`--no-setup` flag that skips the setup command (the copy step still runs). The agent spawn path
defers execution into the agent's window as defined by the `agents-cli` capability.

#### Scenario: Configured setup runs in the new worktree

- **WHEN** a worktree is created in a repository whose effective configuration has a
  `[worktree] setup` command
- **THEN** the system runs that command with the working directory set to the new worktree and
  `BIRDSEYE_WORKTREE`, `BIRDSEYE_REPO`, and `BIRDSEYE_BRANCH` exported

#### Scenario: Repo-local overrides the user default

- **WHEN** both the user config and the repository's `.birdseye/config.toml` set
  `[worktree] setup`
- **THEN** the repository's value is used and the user default is ignored

#### Scenario: No setup configured

- **WHEN** neither the user config nor the repository sets `[worktree] setup`
- **THEN** worktree creation completes with no setup command run

#### Scenario: `be worktree add` streams setup and fails on error

- **WHEN** `be worktree add` creates a worktree and the setup command exits non-zero
- **THEN** the command's output is streamed to the terminal and `be worktree add` exits
  non-zero, leaving the worktree on disk

#### Scenario: `--no-setup` skips the command

- **WHEN** the user runs `be worktree add --no-setup`
- **THEN** the worktree is created and the `.worktreeinclude` copy runs, but the setup command
  does not

#### Scenario: Re-opening an existing worktree does not re-run setup

- **WHEN** an existing worktree is re-opened rather than created
- **THEN** the setup command does not run, because setup is a creation-time step
