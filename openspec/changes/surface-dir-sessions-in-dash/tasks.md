## 1. Extract `internal/sessname`

- [x] 1.1 Create `internal/sessname` with `Sanitize(base)`, `For(base, identity)` (= `Sanitize(base) + "-" + sha256(identity)[:6]`), and `Home(gitCommonDir)` (= `For(repoBase(gitCommonDir), gitCommonDir)`), porting the folding rules from `dir.SessionName` and the repo-base derivation from `dir.HomeSession`.
- [x] 1.2 Add a test asserting `sessname.Home(x)` equals the pre-refactor `dir.HomeSession(x)` for representative git-common-dirs (the behavior-preserving guard), plus tests for `For` determinism and collision-distinctness (`~/a/v3` vs `~/b/v3`, and `v.3`/`v:3`/`v 3`).

## 2. Route all naming through `sessname`

- [x] 2.1 Point `internal/worktree` and `internal/fleet` at `sessname.Home` instead of `dir.HomeSession`; update imports so `agents`, `fleet`, and `cli` import `sessname`, not `providers/dir`, for naming.
- [x] 2.2 Remove `dir.HomeSession`/`dir.SessionName` (or reduce to thin `sessname` wrappers), and confirm no core package still imports `providers/dir` solely for naming.
- [x] 2.3 Switch the `dir` provider (`dir.go`) to name candidates `sessname.For(base, cleanPath)`; keep `Ensure(name, path, base)` + `Connect(name)` behavior.
- [x] 2.4 Verify the collision fix end to end: two distinct directories sharing a basename both appear in `be sessions` and open into different sessions (previously the second was dropped by registry dedup).

## 3. `RowDir` in the reconciler

- [x] 3.1 Add `RowDir` to `RowKind` in `agent.go`; document it as a jump-only non-git directory location, and confirm `isManagedWorktree()`/spawn predicates already exclude it (`Worktree == ""`, `GitDir == ""`).
- [x] 3.2 In `Workspace.Rows()`, track per session during the pane loop whether any pane resolved to a repository and whether any pane hosts an agent; after repo sections and incidental-agent rows, emit one `RowDir` per session with panes, no repo hit, and no agent - `SessionID = "dir:"+session`, `Dir` = the deterministic representative pane's `StartPath` (lowest window index then pane), `TmuxSession`/`TmuxPane` set for jump/preview.
- [x] 3.3 Unit-test the reconciler: a pure non-git agentless session yields exactly one `RowDir`; a session with a repo pane (or an agent) yields no `RowDir`; a closed directory yields nothing.

## 4. View: render and gate

- [x] 4.1 Add a `theme` token (glyph, and color if needed) for the directory row, distinct from the anchor `⌂` and slot `◌` glyphs; reference it from the view (no hardcoded glyph/color, per the theme guard test).
- [x] 4.2 Render `RowDir` in the Projects lens as a one-row entry: directory basename with the path in the detail column; support Enter (jump) and preview.
- [x] 4.3 Gate verbs: spawn (`n`) on a `RowDir` reports "not a git repository" (no worktree created); decide and wire `dd` (close-session-with-confirm vs disabled) per the design's open question.

## 5. Verify and document

- [x] 5.1 Run the repo quality checks (`just` / test suite, theme guard, vet).
- [x] 5.2 Drive the TUI with the use-tty skill: open a non-git directory via `be sessions`, confirm it appears in `be dash` as a jump-only row, Enter jumps, preview renders, spawn is rejected, and the glyph renders correctly in a raw terminal.
- [x] 5.3 Confirm the linked follow-up (directory/worktree overlap) is captured as a separate change and referenced from this one; do not implement it here.
