## Why

Managed-repo recognition is too loose: a tmux session is flagged managed (and drawn with
the managed indicator) when **any single** window started inside **any** git worktree. For
a developer who works inside git repos, that lights up essentially every session, and a
session spanning several different repos gets claimed by whichever repo wins a tiebreak —
contradicting the design intent that a managed session is a dedicated, single-repo
worktree set, not "any session that happens to touch git." Separately, the same concept is
named two ways across surfaces — the dash labels the primary-worktree row `base` while the
headless `--json` contract reports `kind: "anchor"` — which is needless friction.

## What Changes

- **BREAKING (recognition):** Tighten managed-repo recognition. A session SHALL be managed
  only when **every** pane resolves to a git worktree of the **same** repository (one
  `git-common-dir`). Any pane outside a git worktree, or in a different repository,
  disqualifies the whole session — it renders as a plain session. This replaces the
  "at least one worktree window" rule.
- Drop the "incidental agent / non-worktree window inside a managed session" concept: under
  the stricter rule such a window cannot coexist with managed status, so a managed session
  contains only the one repo's worktree rows (anchor + worktrees/slots).
- **BREAKING (contract):** Rename the work `kind` value `anchor` → `base` so the headless
  `--json` contract matches the dash's `base` label. The primary-worktree concept is named
  `base` across all user-facing surfaces (TUI, `--json`, shipped skills, docs). Cheap to do
  now — the `kind` contract only shipped in the just-merged `agents-cli` and has no known
  consumers.
- Change the managed-repo indicator glyph from `󰊢` (U+F02A2) to `󱘎` (U+F160E).

## Capabilities

### New Capabilities

(none)

### Modified Capabilities

- `agent-view`: "Managed-repo recognition" — eligibility changes from "≥1 worktree window"
  to "every pane is a worktree of the same repo"; the incidental-agent-in-managed-session
  scenarios are removed.
- `agents-cli`: "Data verbs emit a stable JSON contract" — the `kind` vocabulary value
  `anchor` becomes `base`.

## Impact

- Code: `internal/agents/workspace.go` (`managedRepoOf`, and the incidental-agent branch in
  `Rows`); `internal/agents/view.go` (`managedGlyph` constant); `internal/fleet/fleet.go`
  (`kindOf` returns `"base"`).
- Shipped skills: `internal/agents/workflows/claude/skills/birdseye-{orchestrator,qa,pr}/SKILL.md`
  (kind/term references `anchor` → `base`).
- Docs: `README.md`, `DESIGN.md` references to `anchor`.
- Contract: `be agents list/status --json` `kind` field value changes for primary
  worktrees (`anchor` → `base`).
- No new dependencies. Internal Go identifiers (`RowAnchor`) are unaffected — not
  user-facing.
