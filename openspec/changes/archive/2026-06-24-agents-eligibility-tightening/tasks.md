## 1. Tighten recognition

- [x] 1.1 Rewrite `managedRepoOf` in `internal/agents/workspace.go` to a per-pane test: a
      session is managed iff every pane resolves to a git worktree and all panes share one
      `git-common-dir`; any non-git or foreign-repo pane returns `ok=false`. Pick the
      single repo's `RepoInfo` (primary worktree as anchor) from the agreed set.
- [x] 1.2 Remove the now-dead incidental-agent branch in `Workspace.Rows` (the loop that
      folds non-worktree agent windows into a managed session) and any helper it alone used.
- [x] 1.3 Update/replace the unit tests in `internal/agents/workspace_test.go` (or the
      relevant test file): a single-repo session is managed; a session with a non-git pane
      is plain; a session with panes in two repos is plain; a no-worktree session is plain;
      same-repo multi-worktree session stays managed.

## 2. Rename kind to base

- [x] 2.1 Change `kindOf` in `internal/fleet/fleet.go` to return `"base"` for
      `RowAnchor` instead of `"anchor"`.
- [x] 2.2 Update fleet tests asserting the `kind` value to expect `"base"`.
- [x] 2.3 Swap `anchor` → `base` in the shipped skills
      `internal/agents/workflows/claude/skills/birdseye-{orchestrator,qa,pr}/SKILL.md`
      (handle/term references describing the primary worktree).
- [x] 2.4 Swap user-facing `anchor` → `base` references in `README.md` and `DESIGN.md`.

## 3. Swap the managed glyph

- [x] 3.1 Change `managedGlyph` in `internal/agents/view.go` from `"\U000f02a2"` to
      `"\U000f160e"` and update its comment to `󱘎`.

## 4. Verify

- [x] 4.1 Run `just check` (build, gofmt/lint, tests, theme guard) and fix any fallout.
- [x] 4.2 Run `openspec validate agents-eligibility-tightening` and confirm it passes.
