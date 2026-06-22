## MODIFIED Requirements

### Requirement: Clone into managed layout

The system SHALL clone a repository into `<parent>/<repo>/<default-branch>`, where
`<parent>` is the optional second argument (default: the current directory),
`<repo>` is derived from the clone URL, and `<default-branch>` is the repository's
actual default branch resolved from `origin/HEAD` (e.g. `main`, `master`, `trunk`).
This establishes a `<repo>/` container under which sibling worktrees live, without
requiring any common configured root.

#### Scenario: Clone creates container layout in the current directory

- **WHEN** the user runs `be worktree clone <url>` with no parent argument
- **THEN** the repository is placed at `<cwd>/<repo>/<default-branch>` as a normal
  checkout, where `<default-branch>` is the repository's resolved default branch

#### Scenario: Clone creates container layout under an explicit parent

- **WHEN** the user runs `be worktree clone <url> <parent>`
- **THEN** the repository is placed at `<parent>/<repo>/<default-branch>` as a normal
  checkout from which worktrees can be created

#### Scenario: Repo already cloned

- **WHEN** `<parent>/<repo>/<default-branch>` already exists
- **THEN** the system does not re-clone and reports the existing layout

### Requirement: Create sibling worktrees

The system SHALL create additional git worktrees as siblings of the default-branch
checkout under the same `<repo>/` container, inferring the container from the current
directory rather than from a configured root. The container is the parent of the
worktree returned by `git rev-parse --show-toplevel`, so `be worktree add` works from
anywhere inside any worktree of the repo. Branch handling SHALL be intuitive: with no
branch argument a new branch named after the worktree is created off the default
branch; a worktree name that matches an existing local or remote branch checks that
branch out; an explicit branch argument is used as given.

#### Scenario: Add a worktree from inside the repo

- **WHEN** the user runs `be worktree add <name>` from any directory inside a managed
  repo's worktree
- **THEN** the system creates `<container>/<name>` as a git worktree, where
  `<container>` is the parent of the current worktree's top level

#### Scenario: Add with no branch creates a new branch

- **WHEN** the user runs `be worktree add <name>` and no branch named `<name>` exists
- **THEN** the system creates a new branch `<name>` off the default branch and checks
  it out in the new worktree

#### Scenario: Add reuses an existing branch

- **WHEN** the user runs `be worktree add <name>` and `<name>` matches an existing
  local or remote branch (or an explicit branch argument is given)
- **THEN** the system checks that branch out in the new worktree rather than failing

#### Scenario: Add outside a managed repo

- **WHEN** the user runs `be worktree add <name>` from a directory that is not inside
  a git worktree
- **THEN** the system reports that it must be run from inside a repo and exits non-zero

#### Scenario: Duplicate worktree name

- **WHEN** a worktree with the requested name already exists in the container
- **THEN** the system declines to overwrite it and reports the conflict

### Requirement: Worktrees as session candidates

The system SHALL expose worktrees discovered under an optional, configurable list of
search paths (`[worktree] roots`) as session-provider candidates, so selecting one
opens a tmux session rooted in that worktree directory. There is no single mandatory
root; when no roots are configured the provider yields no candidates and worktrees
remain usable through the CLI.

#### Scenario: Worktrees appear in picker

- **WHEN** one or more `roots` are configured and each contains repositories following
  the `<repo>/<default-branch>` structure with worktrees, and the provider is enabled
- **THEN** the picker lists each repository's default-branch checkout and worktrees as
  create/attach candidates

#### Scenario: No roots configured

- **WHEN** the worktree provider is enabled but no `roots` are configured
- **THEN** the provider yields no candidates and does not error

#### Scenario: Selecting a worktree opens its session

- **WHEN** the user selects a worktree candidate
- **THEN** the system ensures a tmux session rooted in that worktree directory and
  connects to it via the tmux backend
