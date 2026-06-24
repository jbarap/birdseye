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

### Requirement: Worktrees as session candidates

The system SHALL expose worktrees discovered under an optional, configurable list of
search paths (`[worktree] roots`) as session-provider candidates, so selecting one opens
a tmux session rooted in that worktree directory. Discovery SHALL find git repositories
under the configured roots and enumerate each repository's worktrees from git
(`git worktree list`), rather than matching a `<repo>/<default-branch>` path shape. There
is no single mandatory root; when no roots are configured the provider yields no
candidates and worktrees remain usable through the CLI.

#### Scenario: Worktrees appear in picker

- **WHEN** one or more `roots` are configured, each containing git repositories, and the
  provider is enabled
- **THEN** the picker lists each repository's worktrees (enumerated from git) as
  create/attach candidates, regardless of their on-disk layout

#### Scenario: No roots configured

- **WHEN** the worktree provider is enabled but no `roots` are configured
- **THEN** the provider yields no candidates and does not error

#### Scenario: Selecting a worktree opens its session

- **WHEN** the user selects a worktree candidate
- **THEN** the system ensures a tmux session rooted in that worktree directory and
  connects to it via the tmux backend

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
  under any bird's-eye layout convention
- **THEN** git enumeration still reports it and the system recognizes it as a worktree of
  its repository

