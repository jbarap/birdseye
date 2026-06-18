## ADDED Requirements

### Requirement: Clone into managed layout

The system SHALL clone a repository into a `<repo>/main` working directory so that sibling
worktrees can live alongside it under the same `<repo>/` parent.

#### Scenario: Clone creates main layout

- **WHEN** the user clones a repository via `be worktree`
- **THEN** the repository is placed at `<repo>/main` and is a normal checkout from which
  worktrees can be created

#### Scenario: Repo already cloned

- **WHEN** `<repo>/main` already exists
- **THEN** the system does not re-clone and reports the existing layout

### Requirement: Create sibling worktrees

The system SHALL create additional git worktrees as siblings of `main` (e.g. `<repo>/wt1`,
`<repo>/wt2`) tied to a branch, reusing git's worktree mechanism.

#### Scenario: Add a worktree

- **WHEN** the user requests a new worktree with a name and branch
- **THEN** the system creates `<repo>/<name>` as a git worktree on that branch

#### Scenario: Duplicate worktree name

- **WHEN** a worktree with the requested name already exists
- **THEN** the system declines to overwrite it and reports the conflict

### Requirement: Worktrees as session candidates

The system SHALL expose managed repositories' `main` and worktrees as session-provider
candidates so selecting one opens a tmux session rooted in that worktree directory.

#### Scenario: Worktrees appear in picker

- **WHEN** managed repositories with worktrees exist and the provider is enabled
- **THEN** the picker lists each `main` and worktree as a create/attach candidate

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
