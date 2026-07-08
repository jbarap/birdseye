## Context

`be dash` derives its rows fresh every tick from live tmux panes + agent hook records + git facts
(`internal/agents/workspace.go`). Nothing about the view is persisted except the user's per-location
mute intent. Two lenses (Agents triage, Projects topology) are projections of one row set. Today
every row is shown; a user with a mix of personal and work agents cannot slice by life-context.

Constraints that shape this design:

- The dashboard is a **triage** tool; its core value is surfacing a `needs-attention` agent. Any
  slicing that can fully hide a blocked agent undermines that.
- Rows are derived, not stored. A namespace must be **derivable**, not a per-agent field.
- Non-agent rows (`⌂ base` anchors, `◌ slot`s) are synthesized by the reconciler and carry no agent
  record, but they do carry `GitDir`. Any repo-level attribute must key on `GitDir`.
- Height accounting is tight: the sidebar's two lens panes plus the preview must not overflow the
  terminal (established by the sidebar change). New chrome costs measured rows.

## Goals / Non-Goals

**Goals:**

- Slice the dashboard by a path-derived namespace, switchable via a top tab bar and `]`/`[`.
- Keep the two lenses consistent by filtering **once, upstream** of the projections.
- Never fully hide urgency: a hidden namespace with a `needs-attention` agent dots its tab.
- Be inert (no tab bar, no behavior change) until a namespace is configured.
- Rename the Workspaces lens to Projects, freeing "workspace" for the namespace layer.

**Non-Goals:**

- Per-repo manual namespace assignment / override store (a possible later addition; out of scope).
- Many-to-many namespace membership or arbitrary tags. A repo maps to at most one namespace.
- Persisting the active tab across `be dash` restarts (opens on `All` every time).
- Namespacing incidental (non-repo) agents; they live only under `All`.

## Decisions

### Namespace membership is path-derived, keyed by repository

A namespace is `{name, roots}` in config. A repository maps to the namespace whose configured root
contains the repository's path; ties break to the longest matching root (most specific). Membership
is computed each tick from `GitDir`/repo path, never stored.

- **Why not per-agent state:** the namespace is a property of the *project*, not the agent. Two
  agents in one repo must share it, and anchor/slot rows (no agent) must get it too. Storing
  per-agent would split a repo across tabs and orphan the agentless rows. `Muted` is the precedent
  for "user intent keyed at the right grain" - namespace's grain is the repository (`GitDir`).
- **Why path-derived over a per-repo store (for now):** users already separate personal/work on
  disk, so roots reuse the exact `[repo].roots`/`[dir].roots` conventions with zero new bookkeeping
  and automatic pickup of new repos. A per-repo override store can layer on later without changing
  this contract.
- **Alternative considered:** explicit `{name, repos: [...]}` lists - rejected as more maintenance
  for the common directory-aligned case.

### Unmatched repositories fall back to a parent-derived workspace (default-on)

A recognized repository under no configured root is assigned to an automatic workspace keyed by its
parent directory and labelled with that directory's base name, so unconfigured repositories are
grouped rather than confined to `All`. This is on by default whenever namespaces are configured -
a hidden repository under its parent is strictly more discoverable than one only reachable via
`All`, and off-by-default would mean most users never find it.

- **Why the tab list is keyed, not indexed:** the active tab is tracked by a stable string key
  (`""`=All, a configured name, or an `autoPrefix`-tagged parent path), not a positional index.
  Automatic tabs come and go with the live rows; an index would silently point at a different tab
  when the tail changes. A key is stable, and an active automatic tab that empties cleanly falls
  back to `All`.
- **Why configured-first, auto-sorted-after:** configured tabs are declared, so they are always
  present and hold fixed positions; only the volatile automatic tail (sorted by label) moves as
  repositories appear and leave, so a configured tab never shifts under the user.
- **Why gated on configured namespaces:** automatic workspaces form only when at least one
  `[[workspace]]` exists, preserving the inert default - no config still means no tab bar.
- **Trade-off:** two parents sharing a base name yield two same-labelled tabs (keyed by distinct
  paths, so filtering stays correct); a cosmetic collision, and declaring a `[[workspace]]` to
  group them is the escape hatch.

### The filter is applied once, upstream of the lenses

The active namespace filters the reconciled row set before it is handed to the two projections, so
Agents and Projects are always slices of the same set. `All` is a pass-through pseudo-namespace and
the default.

```
 Workspace.Rows()  ──►  namespaceFilter(active)  ──►  Agents projection
   (unchanged)              rows w/ mapped repo   └──►  Projects projection
                            All = identity
```

- **Why upstream:** filtering each lens independently risks divergence (an agent in one lens, gone
  from the other). One filter guarantees the "one agent, one row in each lens" invariant already in
  the dash-lenses spec.
- **Incidental agents** have no repo → no namespace → dropped by any named filter, kept by `All`.

### Tab bar is chrome, urgency dots computed from the unfiltered set

The tab bar renders `All` + configured namespaces in config order, one `chromeLine` at the top,
active tab in `theme.Accent` + a glyph/bracket (never color alone). Per-tab urgency dots are
computed by scanning the **unfiltered** row set grouped by namespace for a non-muted
`needs-attention` agent - cheap, and independent of which tab is active.

- **Why dots:** they are the safety valve that makes an exclusive-tab model acceptable for a triage
  tool - a blocked agent in a hidden world still pokes through.
- Muted agents do not raise a dot, mirroring the Projects section-badge muting rule.

### Inert when unconfigured

With zero namespaces configured, no tab bar renders, no filter runs, and `chromeLines()` is
unchanged - the dashboard is byte-for-byte today's. This keeps the feature strictly opt-in.

### Rename Workspaces lens → Projects

A terminology-only swap across the view, config (`default.toml` prose), and the dash-lenses spec.
It removes the standing Workspaces/worktree collision and frees "workspace" for the namespace. The
dash-lenses delta also reconciles the stale left/right geometry in "Lens focus and linked
selection" to the stacked sidebar, since that requirement is edited for the rename anyway.

## Risks / Trade-offs

- **[Exclusive tabs can hide urgency]** → urgency dots on non-active tabs; `All` is the default and
  home base, so hiding is opt-in per switch, never the resting state.
- **[Ambiguous membership when roots nest]** → deterministic longest-match rule, covered by a spec
  scenario.
- **[Tab bar costs a row]** → folded into `chromeLines()`; only spent when namespaces are
  configured, and validated against terminal height by existing "fits terminal height" tests.
- **[Rename churn in dash-lenses spec]** → done as `RENAMED` headers + `MODIFIED` bodies; verified
  with `openspec validate --strict`. Bundled per the user's request rather than split.

## Open Questions

- Should the urgency dot also reflect `idle` (waiting on user), or stay strict to `needs-attention`?
  Starting strict to keep the signal meaningful.
- Resolved: a repo under no namespace root gets an automatic parent-derived workspace tab
  (default-on), rather than being `All`-only - see the decision above.
