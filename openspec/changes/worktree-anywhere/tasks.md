## 1. Worktree core (internal/worktree/worktree.go)

- [ ] 1.1 Add default-branch resolution: a helper that returns a repo's default branch
  from `origin/HEAD` (clone-time via `git ls-remote --symref origin HEAD`, post-clone
  via `git symbolic-ref refs/remotes/origin/HEAD`), falling back to the anchor's
  checked-out branch when no remote is resolvable.
- [ ] 1.2 Rework `Clone` to `Clone(parent, url)` writing `<parent>/<repo>/<default-branch>`;
  default `parent` to cwd at the call site; keep the no-op-if-exists behavior reporting
  the existing layout.
- [ ] 1.3 Add container inference: a helper that, given a starting dir, returns the
  `<repo>/` container as the parent of `git rev-parse --show-toplevel`, with a clear
  error when not inside a worktree.
- [ ] 1.4 Rework `Add` to `Add(cwd, name, branch)` (container inferred from cwd):
  create `<container>/<name>`; branch semantics — new branch `<name>` off default when
  none given and `<name>` is not an existing branch, check out an existing local/remote
  branch matching `<name>`, or use an explicit `branch` arg; decline on duplicate dir.
- [ ] 1.5 Generalize `List` to scan a set of roots and recognize a container by "has a
  child checked out on the repo's default branch" rather than a child literally named
  `main`.

## 2. Config (internal/config)

- [ ] 2.1 Replace `Worktree.Root string` (`toml:"root"`) with `Worktree.Roots []string`
  (`toml:"roots"`).
- [ ] 2.2 Detect a legacy `[worktree] root` key and fail loading with a clear message
  pointing to `roots` (do not silently ignore).

## 3. Provider (internal/worktree/provider.go)

- [ ] 3.1 Change `NewProvider(root string)` to `NewProvider(roots []string)`; scan each
  root via `List`, return no candidates (no error) when empty.
- [ ] 3.2 Update candidate labels/session names for the `<repo>/<default-branch>` layout.

## 4. CLI (internal/cli)

- [ ] 4.1 Update `newWorktreeCmd`: `clone <url> [parent]` (parent optional, defaults to
  cwd) and `add <name> [branch]` (no `<repo>` arg; container inferred from cwd); drop or
  repurpose the `--root` flag.
- [ ] 4.2 Update provider registration in `cli.go` to pass `cfg.Worktree.Roots`.

## 5. Docs & tests

- [ ] 5.1 Update `README.md` (`be worktree` usage, `[worktree] roots`, removal of single
  root) and any `config.toml` example.
- [ ] 5.2 Update/extend `internal/worktree/worktree_test.go` for default-branch
  resolution, container inference, the new `Clone`/`Add`/`List` signatures, and the
  add branch-semantics cases.
- [ ] 5.3 `just check` is green (go vet + go test ./...).
