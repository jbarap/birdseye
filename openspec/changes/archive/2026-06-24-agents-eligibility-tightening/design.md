## Context

`managedRepoOf` (`internal/agents/workspace.go`) currently flags a session managed when the
first pane it scans resolves to a git worktree, tiebreaking between repos (primary-worktree
window wins, else lowest window index). `worktree.Resolve` succeeds for *any* directory
inside *any* git repo, with no notion of configured roots or a layout convention. The net
effect: every session that touches git is "managed," and a multi-repo session is silently
claimed by one repo while its other windows fall through the incidental-agent branch in
`Rows`. The session header's managed flag (`groupRows`) is true if *any* row is managed, so
the `󰊢` indicator paints broadly.

Two surfaces name the primary worktree differently: the TUI gutter word is `base`
(`anchorWord`), the headless `--json` `kind` is `anchor` (`kindOf`).

## Goals / Non-Goals

**Goals:**
- A session is managed only when it is, end to end, one repository's worktree set.
- One user-facing name (`base`) for the primary-worktree concept across TUI, `--json`, the
  shipped skills, and docs.
- Swap the managed-repo indicator glyph to `󱘎` (U+F160E).

**Non-Goals:**
- No opt-in/marker-based eligibility (tmux option, env, "be created this") — recognition
  stays purely structural and git-derived.
- No grouped-sibling-layout requirement — a session of plain same-repo worktrees still
  qualifies; we are not pinning recognition to a `<root>/<repo>/<worktree>` path shape.
- No rename of internal Go identifiers (`RowAnchor`, etc.); they are not user-facing.

## Decisions

### Eligibility: every pane, one repo

`managedRepoOf` changes from "first resolving pane wins" to a two-pass test over **all**
panes of the session:

1. Resolve every pane's start path. If **any** pane does not resolve to a git worktree →
   the session is **not** managed (return `ok=false`).
2. Collect the distinct `git-common-dir` across panes. If there is **more than one** → not
   managed. If exactly one → managed, and the chosen `RepoInfo` is that repo (its primary
   worktree is the anchor regardless of which pane surfaced it).

Checked **per pane**, not per window. tmux panes inherit their window's directory at
creation, so in practice panes of a worktree window share that worktree; per-pane is the
strict, simplest reading and matches the design intent that a managed session is wholly one
repo's worktrees. A window holding panes in two *different worktrees of the same repo* is
still fine — the test keys on `git-common-dir`, not on the worktree.

Consequence: the incidental-agent branch in `Rows` (windows in a managed session that are
not worktrees of its repo) becomes dead — such a window now disqualifies the whole session
before it is ever classified managed. Remove that branch and its delta-spec scenarios.

### Naming: `base` everywhere

`kindOf` returns `"base"` instead of `"anchor"`. The shipped skills
(`birdseye-orchestrator/qa/pr`) and `README.md` / `DESIGN.md` swap the term `anchor` →
`base` in user-facing text. The TUI already says `base`, so it is untouched. Internal
`RowAnchor` and friends keep their names — renaming them is churn with no user-visible
payoff, and the `kind` mapping lives in one place (`kindOf`).

This breaks the `kind` string in `be agents list/status --json`. The contract only shipped
in the just-merged `agents-cli` and has no known external consumers, so the additive-only
stability promise is honored in spirit by flipping it now rather than carrying two names.

### Glyph

`managedGlyph` in `internal/agents/view.go` changes from `"\U000f02a2"` (`󰊢`) to
`"\U000f160e"` (`󱘎`). Pure constant swap; the spec does not pin codepoints, so no spec
delta. Rendering is judged in a raw terminal, not the TUI, per project convention.

## Risks / Trade-offs

- **Stricter recognition can surprise.** A user who keeps a scratch shell window (in `$HOME`
  or a non-git dir) inside an otherwise all-worktree session will see that session drop to
  plain. This is intended — it is the price of "managed means wholly one repo" — and is
  documented in the recognition scenarios. A birdseye-spawned session is all worktree
  windows, so the common path is unaffected.
- **Per-pane strictness vs. per-window.** Per-pane is harsher in the edge case of a single
  off-repo split pane. Accepted for simplicity and intent-fit; revisit only if it proves
  annoying in practice.
- **Contract break.** Mitigated by recency (no known consumers) and by updating the shipped
  skills in the same change so the first-party consumers stay correct.
