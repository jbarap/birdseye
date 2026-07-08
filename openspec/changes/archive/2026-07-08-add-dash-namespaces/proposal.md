## Why

A user running a mix of personal and work agents sees them all in one flat `be dash`. There is
no way to slice the view by life-context, so triage, browsing, and preview all mix worlds that the
user thinks of as separate. People already keep those worlds in separate directories on disk; the
dashboard should let them switch between those worlds without losing its core promise of surfacing
an agent that needs attention.

## What Changes

- Introduce **namespaces** ("workspaces"): a top-level slice of the dashboard, path-derived from
  configured roots. A repository belongs to the namespace whose root contains its path.
- Add a **tab bar** at the top of `be dash` listing `All` followed by the configured namespaces.
  `]` / `[` cycle the active tab (rebindable). `All` is the leftmost default and passes every row
  through; a named namespace filters the row set to its repositories.
- The namespace filter is applied **once, upstream** of the two lenses, so the Agents and Projects
  lenses stay consistent projections of the same (filtered) row set.
- Each non-active tab shows an **urgency dot** when it contains an agent in `needs-attention`, so a
  blocked agent in a hidden namespace is never fully invisible - preserving the triage promise.
- **Unmatched repositories fall back to an automatic parent-derived workspace** (default-on): a
  repository under no configured root gets an automatic tab named after its parent directory, so
  nothing is orphaned to `All` only; siblings under one parent share it. Configured tabs hold fixed
  positions, automatic tabs sort after them and track the live rows.
- **Zero namespaces configured → no tab bar and no behavior change.** The feature is inert until at
  least one namespace is declared (no automatic workspaces either).
- Incidental agents (not inside any recognized git repository) appear only under `All`.
- Rename the **Workspaces** lens to **Projects** throughout the dashboard, config, and specs, to
  free the word "workspace" for the namespace layer and remove the standing Workspaces/worktree
  vocabulary collision.

## Capabilities

### New Capabilities
- `dash-namespaces`: path-derived namespaces over the dashboard row set - how a repository maps to
  a namespace, the tab bar and its `All` default, `]`/`[` switching, per-namespace urgency dots,
  the upstream filter, and the inert-when-unconfigured rule.

### Modified Capabilities
- `dash-lenses`: the Workspaces lens is renamed to the **Projects** lens (terminology only, same
  repo → worktree topology), and the two lenses SHALL project the namespace-filtered row set rather
  than the full row set.

## Impact

- `internal/agents`: the dashboard model gains an active-namespace + tab-bar chrome row and an
  upstream row filter; `renderRows`/lens plumbing renamed Workspaces → Projects.
- `internal/config`: a new `[[workspace]]` table (name + roots) and its loading/validation; roots
  reuse the existing tilde/glob conventions of `[repo].roots` and `[dir].roots`.
- `internal/keymap` (agents): two new rebindable actions for next/prev namespace (`]` / `[`).
- Height accounting: the tab bar costs one `chromeLine`, folding into existing sidebar/preview math.
- Docs: `default.toml` reference and `DESIGN.md` (tab-bar accent + glyph, never color alone).
