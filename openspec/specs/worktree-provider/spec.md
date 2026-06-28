# worktree-provider Specification

## Purpose

TBD: created by archiving change birds-eye-foundation. Update Purpose after archive.
## Requirements
### Requirement: Create sibling worktrees

The system SHALL create additional git worktrees under a **grouped-sibling layout**: for
a repository cloned at `<path>/<repo>`, worktrees live at
`<path>/<repo>.worktrees/<branch-slug>`, where `<branch-slug>` is a filesystem-safe slug
of the branch name (path separators and other unsafe characters become hyphens, e.g.
`feature/login` → `feature-login`). The container SHALL be derived from git
(`git rev-parse --git-common-dir`), not from the parent of the current checkout, so
`be worktree add` works from anywhere inside any worktree of the repo. Branch handling
SHALL remain intuitive: with no branch argument a new branch named after the worktree is
created off the default branch; a name matching an existing local or remote branch
checks that branch out; an explicit branch argument is used as given.

#### Scenario: Add creates a grouped-sibling worktree

- **WHEN** the user runs `be worktree add <branch>` from any directory inside a managed
  repo cloned at `<path>/<repo>`
- **THEN** the system creates `<path>/<repo>.worktrees/<branch-slug>` as a git worktree,
  where `<branch-slug>` is the filesystem-safe slug of the branch name

#### Scenario: Container derived from git, not from the checkout's parent

- **WHEN** the user runs `be worktree add <branch>` from inside any worktree of the repo
- **THEN** the system resolves the repository from `git rev-parse --git-common-dir` and
  places the new worktree under that repository's `<repo>.worktrees/` directory,
  independent of where the current checkout sits

#### Scenario: Add with no branch creates a new branch

- **WHEN** the user runs `be worktree add <name>` and no branch named `<name>` exists
- **THEN** the system creates a new branch `<name>` off the default branch and checks it
  out in the new grouped-sibling worktree

#### Scenario: Add reuses an existing branch

- **WHEN** the user runs `be worktree add <name>` and `<name>` matches an existing local
  or remote branch (or an explicit branch argument is given)
- **THEN** the system checks that branch out in the new worktree rather than failing

#### Scenario: Add outside a managed repo

- **WHEN** the user runs `be worktree add <name>` from a directory that is not inside a
  git worktree
- **THEN** the system reports that it must be run from inside a repo and exits non-zero

#### Scenario: Slug collision is refused

- **WHEN** the branch slug for a requested worktree resolves to a directory that already
  exists under `<repo>.worktrees/`
- **THEN** the system declines to overwrite it and reports the conflict

### Requirement: Repositories as session candidates

The system SHALL expose git repositories discovered under an optional, configurable list
of search paths (`[repo] roots`) as session-provider candidates — **one candidate per
repository** — so selecting one opens a tmux session rooted in that repository's primary
worktree. A repository's linked worktrees SHALL NOT appear as separate top-level
candidates; under the sessions-represent-repos model they are managed as windows within
the repository's session by the agent view. Discovery SHALL find git repositories directly
under the configured roots, deriving each repository's primary worktree from git, rather
than matching a `<repo>/<default-branch>` path shape. There is no single mandatory root;
when no roots are configured the provider yields no candidates and repositories remain
usable through the CLI.

#### Scenario: Repositories appear in picker

- **WHEN** one or more `roots` are configured, each containing git repositories, and the
  provider is enabled
- **THEN** the picker lists one create candidate per repository, regardless of its on-disk
  layout, each opening a session at that repository's primary worktree

#### Scenario: Linked worktrees are not separate candidates

- **WHEN** a discovered repository has additional linked worktrees
- **THEN** the picker still lists only the single repository candidate, not one entry per
  worktree

#### Scenario: No roots configured

- **WHEN** the repository provider is enabled but no `roots` are configured
- **THEN** the provider yields no candidates and does not error

#### Scenario: Selecting a repository opens its session

- **WHEN** the user selects a repository candidate
- **THEN** the system ensures a tmux session rooted in the repository's primary worktree
  and connects to it via the tmux backend

### Requirement: Git availability

The system SHALL require `git` for worktree operations and report clearly when it is
absent rather than failing obscurely.

#### Scenario: git not installed

- **WHEN** a worktree operation is requested and `git` is not on PATH
- **THEN** the system reports that git is required and exits non-zero

### Requirement: Git-native worktree enumeration

The system SHALL enumerate a repository's worktrees from git itself —
`git worktree list --porcelain` keyed by the repository's shared git directory
(`git rev-parse --git-common-dir`) — rather than from any on-disk path convention. The
shared git directory SHALL serve as the stable identity of a repository, identical
across all of its worktrees. Enumeration SHALL recognize every worktree git reports,
including worktrees created by a user or another tool and worktrees placed anywhere on
disk.

#### Scenario: Worktrees enumerated from git

- **WHEN** the system needs a repository's worktree set
- **THEN** it reads `git worktree list` for that repository and returns every worktree
  git reports, regardless of where each worktree lives on disk

#### Scenario: Repository identity is the shared git directory

- **WHEN** the system compares two worktrees to decide whether they belong to the same
  repository
- **THEN** it treats them as the same repository iff they share the same
  `git rev-parse --git-common-dir`, independent of their directory layout

#### Scenario: Third-party worktree is recognized

- **WHEN** a worktree was created by some tool other than `be worktree add` and is not
  under any birdseye layout convention
- **THEN** git enumeration still reports it and the system recognizes it as a worktree of
  its repository

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

